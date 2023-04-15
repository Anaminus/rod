package rod

import (
	"bufio"
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
		// XX XX XX XX XX XX XX XX  XX XX XX XX XX XX XX XX #................#
		// 0                                                 5               6
		// 0                                                 0               6
		const hexoff = 0
		const asciioff = 50
		var buf = make([]byte, 67)
		for i := range buf {
			buf[i] = rSpace
		}
		buf[49] = byte(rInlineComment)
		buf[66] = byte(rInlineComment)
		for line := 0; line < (len(v)-1)/16+1; line++ {
			e.newline()
			for n := 0; n < 16; n++ {
				i := line*16 + n
				j := hexoff + n*3
				k := asciioff + n
				if n >= 8 {
					j++
				}
				if i < len(v) {
					hex.Encode(buf[j:], v[i:i+1])
					buf[k] = toChar(v[i])
				} else {
					buf[j] = rSpace
					buf[j+1] = rSpace
					buf[k] = rSpace
				}
			}
			e.w.Write(buf)
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
		return typeIndex(keys[i]) < typeIndex(keys[j])
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
	case []any:
		return 7
	case map[any]any:
		return 8
	case map[string]any:
		return 9
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
