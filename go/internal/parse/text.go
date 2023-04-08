package parse

import (
	"bufio"
	"io"
	"io/ioutil"
	"sort"
)

// LineReader wraps an io.Reader to keep track of lines.
type LineReader struct {
	R io.Reader // Underlying reader.

	n     int64
	lines []int64
}

// NewLineReader returns a LineReader initialized with Line and Column set to 1.
func NewLineReader(r io.Reader) *LineReader {
	return &LineReader{R: r, lines: []int64{0}}
}

// Read reads from R, keeping track of when newlines are encountered.
func (l *LineReader) Read(p []byte) (n int, err error) {
	n, err = l.R.Read(p)
	if 0 <= n && n <= len(p) {
		b := p[:n]
		for i, c := range b {
			if c == '\n' {
				l.lines = append(l.lines, l.n+int64(i)+1)
			}
		}
	}
	l.n += int64(n)
	return n, err
}

func searchInts(a []int64, x int64) int {
	return sort.Search(len(a), func(i int) bool { return a[i] > x }) - 1
}

// Position returns the line and column from a byte offset.
//
// BUG: Column is in units of bytes rather than characters.
func (r *LineReader) Position(offset int64) (line, column int) {
	if i := searchInts(r.lines, offset); i >= 0 {
		return i + 1, int(offset - r.lines[i] + 1)
	}
	return -1, -1
}

// TextReader wraps an io.Reader to provide primitive methods for parsing text.
type TextReader struct {
	r   *bufio.Reader
	buf []byte
	n   int64
	err error
}

// NewTextReader returns a TextReader that reads r.
func NewTextReader(r io.Reader) *TextReader {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &TextReader{
		r:   br,
		buf: make([]byte, 0, 64),
	}
}

// N returns the number of bytes read from the underlying reader.
func (r *TextReader) N() int64 {
	return r.n
}

// Err returns the first error that occurred while reading, if any.
func (r *TextReader) Err() error {
	return r.err
}

// End returns the number of bytes read, and the first error that occurred.
func (r *TextReader) End() (n int64, err error) {
	return r.n, r.err
}

// Returns and consumes the buffer.
func (r *TextReader) Consume() []byte {
	buf := r.buf
	r.buf = r.buf[:0]
	return buf
}

// Returns the buffer.
func (r *TextReader) Bytes() []byte {
	return r.buf
}

// Next returns the next rune from the reader, and advances the cursor by the
// length of the rune. Returns r < 0 if an error occurred.
func (t *TextReader) Next() (r rune) {
	if t.err != nil {
		return -1
	}
	var w int
	r, w, t.err = t.r.ReadRune()
	if t.err != nil {
		return -1
	}
	t.buf = append(t.buf, string(r)...)
	t.n += int64(w)
	return r
}

// MustNext is like Next, but sets the error to io.ErrUnexpectedEOF if the
// end of the reader is reached.
func (t *TextReader) MustNext() (r rune) {
	if t.err != nil {
		return -1
	}
	if r = t.Next(); r < 0 {
		if t.err == io.EOF {
			t.err = io.ErrUnexpectedEOF
		}
	}
	return r
}

// Peek returns the next rune without advancing the cursor. Returns r < 0 if an
// error occurred.
func (t *TextReader) Peek() (r rune) {
	r, _, t.err = t.r.ReadRune()
	if t.err != nil {
		return -1
	}
	t.r.UnreadRune()
	return r
}

// Is compares s to the next characters in the reader. If they are equal, then
// the cursor is advanced, and true is returned. Otherwise, the cursor is not
// advanced, and false is returned.
func (t *TextReader) Is(s string) (ok bool) {
	if t.err != nil {
		return false
	}
	if s == "" {
		return true
	}
	var b []byte
	if b, t.err = t.r.Peek(len(s)); t.err != nil {
		if t.err == io.EOF {
			t.err = nil
		}
		return false
	}
	if string(b) != s {
		return false
	}
	t.r.Discard(len(s))
	t.buf = append(t.buf, b...)
	t.n += int64(len(b))
	return true
}

// IsRune compares r to the next character in the reader. If they are equal,
// then the cursor is advanced, and true is returned. Otherwise, the cursor is
// not advanced, and false is returned.
func (t *TextReader) IsRune(r rune) (ok bool) {
	if t.err != nil {
		return false
	}
	s := string(r)
	if s == "" {
		return true
	}
	var b []byte
	if b, t.err = t.r.Peek(len(s)); t.err != nil {
		if t.err == io.EOF {
			t.err = nil
		}
		return false
	}
	if string(b) != s {
		return false
	}
	t.r.Discard(len(s))
	t.buf = append(t.buf, b...)
	t.n += int64(len(b))
	return true
}

// IsAny advances the cursor while the next character matches f, or until the
// end of the reader. Returns the characters read, and whether a non-EOF error
// occurred.
func (t *TextReader) IsAny(f func(rune) bool) (ok bool) {
	if t.err != nil {
		return false
	}
	for {
		var c rune
		var w int
		if c, w, t.err = t.r.ReadRune(); t.err != nil {
			if t.err == io.EOF {
				t.err = nil
				return true
			}
			return false
		}
		if !f(c) {
			t.r.UnreadRune()
			return true
		}
		t.buf = append(t.buf, string(c)...)
		t.n += int64(w)
	}
}

// IsEOF returns true if the cursor is at the end of the reader.
func (t *TextReader) IsEOF() (ok bool) {
	if t.err != nil {
		return t.err == io.EOF
	}
	_, err := t.r.Peek(1)
	return err == io.EOF
}

// Skip advances the cursor until a character does not match f. Returns whether
// a non-EOF error occurred.
func (t *TextReader) Skip(f func(rune) bool) (ok bool) {
	if t.err != nil {
		return false
	}
	for {
		var c rune
		var w int
		if c, w, t.err = t.r.ReadRune(); t.err != nil {
			if t.err == io.EOF {
				t.err = nil
				return true
			}
			return false
		}
		if !f(c) {
			t.r.UnreadRune()
			return true
		}
		t.buf = append(t.buf, string(c)...)
		t.n += int64(w)
	}
}

// Until advances the cursor until a character matches v. Returns the characters
// read, and whether an errored occurred.
func (t *TextReader) Until(v rune) (ok bool) {
	if t.err != nil {
		return false
	}
	for {
		var c rune
		var w int
		if c, w, t.err = t.r.ReadRune(); t.err != nil {
			if t.err == io.EOF {
				t.err = io.ErrUnexpectedEOF
			}
			return false
		}
		t.buf = append(t.buf, string(c)...)
		t.n += int64(w)
		if c == v {
			return true
		}
	}
}

// UntilEOL advances the cursor until a character matches end-of-line or
// end-of-file. Returns the characters read, and whether an errored occurred.
func (t *TextReader) UntilEOL() (ok bool) {
	if t.err != nil {
		return false
	}
	for {
		var c rune
		var w int
		if c, w, t.err = t.r.ReadRune(); t.err != nil {
			if t.err == io.EOF {
				t.err = nil
			}
			return true
		}
		t.buf = append(t.buf, string(c)...)
		t.n += int64(w)
		if c == '\n' {
			return true
		}
	}
}

// UntilAny advances the cursor until a character matches f. Returns the
// characters read, and whether an errored occurred.
func (t *TextReader) UntilAny(f func(rune) bool) (ok bool) {
	if t.err != nil {
		return false
	}
	for {
		var c rune
		var w int
		if c, w, t.err = t.r.ReadRune(); t.err != nil {
			if t.err == io.EOF {
				t.err = io.ErrUnexpectedEOF
			}
			return false
		}
		t.buf = append(t.buf, string(c)...)
		t.n += int64(w)
		if f(c) {
			return true
		}
	}
}

// UntilEOF reads the remaining characters in the reader.
func (t *TextReader) UntilEOF() (ok bool) {
	if t.err != nil {
		return false
	}
	var b []byte
	if b, t.err = ioutil.ReadAll(t.r); t.err != nil {
		return false
	}
	t.buf = append(t.buf, b...)
	t.n += int64(len(b))
	return true
}
