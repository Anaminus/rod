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
		e.w.WriteString("null")
	case bool:
		if v {
			e.w.WriteString("true")
		} else {
			e.w.WriteString("false")
		}
	case int64:
		e.w.WriteString(strconv.FormatInt(v, 10))
	case float64:
		if v == math.Inf(1) {
			e.w.WriteString("inf")
		} else if v == math.Inf(-1) {
			e.w.WriteString("-inf")
		} else if v != v {
			e.w.WriteString("nan")
		} else {
			e.w.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		}
	case string:
		e.w.WriteByte('"')
		for _, r := range v {
			switch r {
			case '"', '\\':
				e.w.WriteByte('\\')
			}
			e.w.WriteRune(r)
		}
		e.w.WriteByte('"')
	case []byte:
		if len(v) == 0 {
			e.w.WriteString("||")
			break
		}
		e.push()
		// | XX XX XX XX XX XX XX XX  XX XX XX XX XX XX XX XX |#................#
		// 0 0                                                  5               6
		// 0 2                                                  3               9
		var buf = make([]byte, 70)
		for i := range buf {
			buf[i] = ' '
		}
		buf[0] = '|'
		buf[51] = '|'
		buf[52] = '#'
		buf[69] = '#'
		for line := 0; line < (len(v)-1)/16+1; line++ {
			e.newline()
			for n := 0; n < 16; n++ {
				i := line*16 + n
				j := 2 + n*3
				k := 53 + n
				if n >= 8 {
					j++
				}
				if i < len(v) {
					hex.Encode(buf[j:], v[i:i+1])
					buf[k] = toChar(v[i])
				} else {
					buf[j] = ' '
					buf[j+1] = ' '
					buf[k] = ' '
				}
			}
			e.w.Write(buf)
		}
		e.pop()
		e.newline()
	case []any:
		e.w.WriteByte('[')
		e.push()
		for _, v := range v {
			e.newline()
			if err := e.encodeValue(v); err != nil {
				return err
			}
			e.w.WriteByte(',')
		}
		e.pop()
		e.newline()
		e.w.WriteByte(']')
	case map[any]any:
		e.w.WriteByte('(')
		e.push()
		err := mapForEach(v, func(k, v any) error {
			e.newline()
			//TODO: ensure primitive.
			if err := e.encodeValue(k); err != nil {
				return err
			}
			e.w.WriteString(": ")
			if err := e.encodeValue(v); err != nil {
				return err
			}
			e.w.WriteByte(',')
			return nil
		})
		if err != nil {
			return err
		}
		e.pop()
		e.newline()
		e.w.WriteByte(')')
	case map[string]any:
		e.w.WriteByte('{')
		e.push()
		err := structForEach(v, func(i string, v any) error {
			e.newline()
			if err := e.encodeIdent(i); err != nil {
				return err
			}
			e.w.WriteString(": ")
			if err := e.encodeValue(v); err != nil {
				return err
			}
			e.w.WriteByte(',')
			return nil
		})
		if err != nil {
			return err
		}
		e.pop()
		e.newline()
		e.w.WriteByte('}')
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
