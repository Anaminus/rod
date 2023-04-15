package rod

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Decoder reads and decodes ROD values from an input stream.
type Decoder struct {
	l    *lexer
	next token
	eof  bool
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	d := Decoder{
		l: newLexer(r),
	}
	return &d
}

// Decode decodes a value into v. v must be a pointer to an empty interface.
// Other types are not currently supported.
//
// ROD types are decoded into the following Go types:
//
//     null    : nil
//     bool    : bool
//     integer : int64
//     float   : float64
//     string  : string
//     blob    : []byte
//     array   : []any
//     map     : map[any]any
//     struct  : map[string]any
//
func (d *Decoder) Decode(v any) error {
	a, ok := v.(*any)
	if !ok {
		return errors.New("argument must be pointer to any")
	}
	if err := d.decodeValue(a); err != nil {
		return err
	}

	// Expect EOF.
	d.eof = true
	_, err := d.nextToken()
	return err
}

func (d *Decoder) unexpectedToken(t token) {
	panic(fmt.Errorf("lexer emitted unexpected token %s (%[1]d) at %d-%d",
		t.Type,
		t.Position.StartOffset,
		t.Position.EndOffset,
	))
}

// Gets the next token from the lexer. Expects a non-EOF token. Skips over
// whitespace and comments.
func (d *Decoder) nextToken() (t token, err error) {
	t = d.next
	if t.Type != tInvalid {
		d.next.Type = tInvalid
		return t, nil
	}
retry:
	if !d.l.Next() {
		panic("no more tokens")
	}
	if err := d.l.Err(); err != nil {
		return t, err
	}
	t = d.l.Token()
	switch t.Type {
	case tEOF:
		if d.eof {
			return t, nil
		}
		return t, io.ErrUnexpectedEOF
	case tSpace, tInlineComment, tBlockComment, tAnnotation:
		goto retry
	}
	return t, nil
}

// Peek at the next token. If it matches t, then consume it.
func (d *Decoder) ifToken(t tokenType) bool {
	var err error
	d.next, err = d.nextToken()
	if err != nil {
		return false
	}
	if d.next.Type != t {
		return false
	}
	d.next.Type = tInvalid
	return true
}

// Expects a specific token from the lexer.
func (d *Decoder) expectToken(t tokenType) {
	if token, _ := d.nextToken(); token.Type != t {
		d.unexpectedToken(token)
	}
}

// Decodes one value into a.
func (d *Decoder) decodeValue(a *any) error {
	for {
		t, err := d.nextToken()
		if err != nil {
			return err
		}
		switch t.Type {
		default:
			d.unexpectedToken(t)
		case tNull:
			*a = nil
			return nil
		case tTrue:
			*a = true
			return nil
		case tFalse:
			*a = false
			return nil
		case tInf:
			*a = math.Inf(1)
			return nil
		case tNaN:
			*a = math.NaN()
			return nil
		case tPos:
			return d.decodeNumber(a, 1)
		case tNeg:
			return d.decodeNumber(a, -1)
		case tInteger:
			return d.decodeInteger(a, 1, t.Value)
		case tFloat:
			return d.decodeFloat(a, 1, t.Value)
		case tString:
			return d.decodeString(a, t.Value)
		case tBlob:
			return d.decodeBlob(a)
		case tArrayOpen:
			return d.decodeArray(a)
		case tMapOpen:
			return d.decodeMap(a)
		case tStructOpen:
			return d.decodeStruct(a)
		}
	}
}

// Decodes a numeric value into a with the given sign.
func (d *Decoder) decodeNumber(a *any, sign int) error {
	for {
		t, err := d.nextToken()
		if err != nil {
			return err
		}
		switch t.Type {
		default:
			d.unexpectedToken(t)
		case tInf:
			*a = math.Inf(sign)
			return nil
		case tInteger:
			return d.decodeInteger(a, sign, t.Value)
		case tFloat:
			return d.decodeFloat(a, sign, t.Value)
		}
	}
}

// Decodes an integer from s with the given sign into a as an int64.
func (d *Decoder) decodeInteger(a *any, sign int, s string) error {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic(fmt.Errorf("lexer emitted int token with invalid value %q: %s", s, err))
	}
	*a = v * int64(sign)
	return nil
}

// Decodes a float from s with the given sign into a as a float64.
func (d *Decoder) decodeFloat(a *any, sign int, s string) error {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(fmt.Errorf("lexer emitted float token with invalid value %q: %s", s, err))
	}
	*a = v * float64(sign)
	return nil
}

// Decodes a quoted string from s into a as a string.
func (d *Decoder) decodeString(a *any, s string) error {
	if !strings.HasPrefix(s, string(rString)) || !strings.HasSuffix(s, string(rString)) {
		panic(fmt.Errorf("lexer emitted string token without delimiters"))
	}

	r := strings.NewReader(s[1 : len(s)-1])
	var b strings.Builder
	for {
		c, _, err := r.ReadRune()
		if err == io.EOF {
			break
		}
		switch c {
		case '\\':
			switch c, _, _ := r.ReadRune(); c {
			case 'a':
				b.WriteRune('\a')
			case 'b':
				b.WriteRune('\b')
			case 'f':
				b.WriteRune('\f')
			case 'n':
				b.WriteRune('\n')
			case 'r':
				// Discarded.
			case 't':
				b.WriteRune('\t')
			case 'v':
				b.WriteRune('\v')
			case '\\':
				b.WriteRune('\\')
			case '\'':
				b.WriteRune('\'')
			case '"':
				b.WriteRune('"')
			default:
				return fmt.Errorf("string contains invalid escape `\\%s`", string(c))
			}
		default:
			//TODO: Copy entire sequences of non-escapes at once.
			b.WriteRune(c)
		}
	}
	*a = b.String()
	return nil
}

// Decodes a blob sequence into a.
func (d *Decoder) decodeBlob(a *any) error {
	b := bytes.NewBuffer([]byte{})
	p := make([]byte, 1)
loop:
	for {
		t, err := d.nextToken()
		if err != nil {
			return err
		}
		switch t.Type {
		default:
			d.unexpectedToken(t)
		case tByte:
			if _, err = hex.Decode(p, []byte(t.Value)); err != nil {
				panic(fmt.Errorf("lexer emitted byte token with invalid value %q: %s", t.Value, err))
			}
			b.Write(p)
		case tBlob:
			break loop
		}
	}
	*a = b.Bytes()
	return nil
}

// Decodes an array type of the form []any into a.
func (d *Decoder) decodeArray(a *any) error {
	var varray = []any{}
loop:
	for {
		if d.ifToken(tArrayClose) {
			break loop
		}

		var v any
		if err := d.decodeValue(&v); err != nil {
			return err
		}

		varray = append(varray, v)

		t, err := d.nextToken()
		if err != nil {
			return err
		}
		switch t.Type {
		default:
			d.unexpectedToken(t)
		case tSep:
			if d.ifToken(tArrayClose) {
				break loop
			}
			continue
		case tArrayClose:
			break loop
		}
	}
	*a = varray
	return nil
}

// Decodes a map type of the form map[any]any into a.
func (d *Decoder) decodeMap(a *any) error {
	var vmap = map[any]any{}
loop:
	for {
		if d.ifToken(tMapClose) {
			break loop
		}

		var k any
		if err := d.decodeValue(&k); err != nil {
			return err
		}
		// Lexer ensures that value is a primitive.

		d.expectToken(tAssoc)

		var v any
		if err := d.decodeValue(&v); err != nil {
			return err
		}

		vmap[k] = v

		t, err := d.nextToken()
		if err != nil {
			return err
		}
		switch t.Type {
		default:
			d.unexpectedToken(t)
		case tSep:
			if d.ifToken(tMapClose) {
				break loop
			}
			continue
		case tMapClose:
			break loop
		}
	}
	*a = vmap
	return nil
}

// Decodes a struct type of the form map[string]any into a.
func (d *Decoder) decodeStruct(a *any) error {
	var vstruct = map[string]any{}
loop:
	for {
		t, err := d.nextToken()
		if err != nil {
			return err
		}
		switch t.Type {
		default:
			d.unexpectedToken(t)
		case tStructClose:
			break loop
		case tIdent:
		}

		d.expectToken(tAssoc)

		var v any
		if err := d.decodeValue(&v); err != nil {
			return err
		}

		vstruct[t.Value] = v

		t, err = d.nextToken()
		if err != nil {
			return err
		}
		switch t.Type {
		default:
			d.unexpectedToken(t)
		case tSep:
			if d.ifToken(tMapClose) {
				break loop
			}
			continue
		case tMapClose:
			break loop
		}
	}
	*a = vstruct
	return nil
}
