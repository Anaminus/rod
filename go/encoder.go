package rod

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

type Encoder struct {
	w *bufio.Writer

	lead []byte
}

func NewEncoder(w io.Writer) *Encoder {
	e := &Encoder{
		w: bufio.NewWriter(w),
	}
	return e
}

func (e *Encoder) push() {
	e.lead = append(e.lead, '\t')
}

func (e *Encoder) pop() {
	e.lead = e.lead[:len(e.lead)-1]
}

func (e *Encoder) newline() {
	e.w.WriteByte('\n')
	e.w.Write(e.lead)
}

func (e *Encoder) Encode(v any) error {
	if err := e.encodeValue(v); err != nil {
		return err
	}
	return e.w.Flush()
}

func (e *Encoder) encodeValue(v any) error {
	switch v := v.(type) {
	default:
		return fmt.Errorf("cannot encode type %T", v)
	case nil:
		e.w.WriteString(rNull)
	case bool:
		if v {
			e.w.WriteString(rTrue)
		} else {
			e.w.WriteString(rFalse)
		}
	case int64:
		e.w.WriteString(strconv.FormatInt(v, 10))
	case float64:
		if v == math.Inf(1) {
			e.w.WriteString(rInf)
		} else if v == math.Inf(-1) {
			e.w.WriteByte(byte(rNeg))
			e.w.WriteString(rInf)
		} else if v != v {
			e.w.WriteString(rNaN)
		} else {
			s := strconv.FormatFloat(v, 'f', -1, 64)
			e.w.WriteString(s)
			if strings.IndexRune(s, rDecimal) < 0 {
				// Force decimal.
				e.w.WriteRune(rDecimal)
				e.w.WriteByte('0')
			}
		}
	case string:
		e.w.WriteRune(rString)
		for _, r := range v {
			switch r {
			case rString, rEscape:
				e.w.WriteRune(rEscape)
			}
			e.w.WriteRune(r)
		}
		e.w.WriteRune(rString)
	case []byte:
		e.w.WriteRune(rBlob)
		if len(v) == 0 {
			e.w.WriteRune(rBlob)
			break
		}
		e.push()
		e.newline()

		const width = 16 // Bytes per line.
		const half = 8   // Where to add extra space.
		buf := make([]byte, 2)
		for i := range v {
			if i%width != 0 {
				// Space before each byte except start of line.
				e.w.WriteByte(rSpace)
			}
			if (i+half)%width == 0 {
				// Extra space at half width.
				e.w.WriteByte(rSpace)
			}
			// Write byte.
			hex.Encode(buf, v[i:i+1])
			e.w.Write(buf)

			// At end of a full line, display ASCII as comment.
			if (i+1)%width == 0 {
				e.w.WriteByte(rSpace)
				e.w.WriteRune(rInlineComment)
				for j := i + 1 - width; j < i+1; j++ {
					e.w.WriteByte(toChar(v[j]))
				}
				e.w.WriteRune(rInlineComment)
				// If there's more, add a newline.
				if i+1 < len(v) {
					e.newline()
				}
			}
		}
		// Number of extra bytes in last line.
		if n := width - ((len(v)-1)%width + 1); n > 0 {
			for i := 0; i < n; i++ {
				// Space for each extra byte.
				e.w.WriteByte(rSpace)
				e.w.WriteByte(rSpace)
				e.w.WriteByte(rSpace)
			}
			if n >= half {
				// Extra space at half width.
				e.w.WriteByte(rSpace)
			}
			e.w.WriteByte(rSpace)
			e.w.WriteRune(rInlineComment)
			// Number of bytes in last line.
			if n = len(v) - (width - n); n < 0 {
				// Prevet underflow.
				n = 0
			}
			for j := n; j < len(v); j++ {
				e.w.WriteByte(toChar(v[j]))
			}
			e.w.WriteRune(rInlineComment)
		}

		e.pop()
		e.newline()
		e.w.WriteRune(rBlob)
	case []any:
		e.w.WriteRune(rArrayOpen)
		e.push()
		for _, v := range v {
			e.newline()
			if err := e.encodeValue(v); err != nil {
				return err
			}
			e.w.WriteRune(rSep)
		}
		e.pop()
		e.newline()
		e.w.WriteRune(rArrayClose)
	case map[any]any:
		e.w.WriteRune(rMapOpen)
		e.push()
		err := mapForEach(v, func(k, v any) error {
			e.newline()
			//TODO: ensure primitive.
			if err := e.encodeValue(k); err != nil {
				return err
			}
			e.w.WriteRune(rAssoc)
			e.w.WriteByte(rSpace)
			if err := e.encodeValue(v); err != nil {
				return err
			}
			e.w.WriteRune(rSep)
			return nil
		})
		if err != nil {
			return err
		}
		e.pop()
		e.newline()
		e.w.WriteRune(rMapClose)
	case map[string]any:
		e.w.WriteRune(rStructOpen)
		e.push()
		err := structForEach(v, func(i string, v any) error {
			e.newline()
			if err := e.encodeIdent(i); err != nil {
				return err
			}
			e.w.WriteRune(rAssoc)
			e.w.WriteByte(rSpace)
			if err := e.encodeValue(v); err != nil {
				return err
			}
			e.w.WriteRune(rSep)
			return nil
		})
		if err != nil {
			return err
		}
		e.pop()
		e.newline()
		e.w.WriteRune(rStructClose)
	}
	return nil
}

func toChar(b byte) byte {
	if 32 <= b && b <= 126 {
		return b
	}
	return '.'
}

func mapForEach(m map[any]any, f func(k, v any) error) error {
	keys := make([]any, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		ti := typeIndex(keys[i])
		tj := typeIndex(keys[j])
		if ti == tj {
			return typeCmp(keys[i], keys[j])
		}
		return ti < tj
	})
	for _, key := range keys {
		if err := f(key, m[key]); err != nil {
			return err
		}
	}
	return nil
}

func typeIndex(v any) int {
	switch v.(type) {
	default:
		return 0
	case nil:
		return 1
	case bool:
		return 2
	case int64:
		return 3
	case float64:
		return 4
	case string:
		return 5
	case []byte:
		return 6
	}
}

func typeCmp(i, j any) bool {
	switch i := i.(type) {
	default:
		return false
	case nil:
		return false
	case bool:
		return !i && j.(bool)
	case int64:
		return i < j.(int64)
	case float64:
		return i < j.(float64)
	case string:
		return i < j.(string)
	case []byte:
		return bytes.Compare(i, j.([]byte)) < 0
	}
}

func structForEach(s map[string]any, f func(i string, v any) error) error {
	keys := make([]string, 0, len(s))
	for key := range s {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := f(key, s[key]); err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) encodeIdent(s string) error {
	for i, r := range s {
		if i == 0 {
			if !isLetter(r) {
				return errors.New("invalid identifier")
			}
		} else {
			if !isIdent(r) {
				return errors.New("invalid identifier")
			}
		}
	}
	e.w.WriteString(s)
	return nil
}
