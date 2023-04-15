package rod

import (
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/anaminus/rod/go/internal/parse"
)

// Whether a rune is a digit.
func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

// Whether a rune is a hexadecimal digit.
func isHex(r rune) bool {
	return isDigit(r) || ('A' <= r && r <= 'F') || ('a' <= r && r <= 'f')
}

// Whether a rune is a unicode letter or underscore.
func isLetter(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

// Whether a rune is the general protion of an identifier.
func isIdent(r rune) bool {
	return isLetter(r) || isDigit(r)
}

// Rune and string constants.
const (
	rError           rune   = -1
	rSpace           byte   = ' '
	rInlineComment   rune   = '#'
	rBlockComment    string = "#<"
	rBlockCommentEnd rune   = '>'
	rAnnotation      rune   = '<'
	rAnnotationEnd   rune   = '>'
	rNull            string = "null"
	rTrue            string = "true"
	rFalse           string = "false"
	rInf             string = "inf"
	rNaN             string = "nan"
	rPos             rune   = '+'
	rNeg             rune   = '-'
	rDecimal         rune   = '.'
	rString          rune   = '"'
	rEscape          rune   = '\\'
	rBlob            rune   = '|'
	rSep             rune   = ','
	rAssoc           rune   = ':'
	rArrayOpen       rune   = '['
	rArrayClose      rune   = ']'
	rMapOpen         rune   = '('
	rMapClose        rune   = ')'
	rStructOpen      rune   = '{'
	rStructClose     rune   = '}'
)

// Indicates the type of token emitted.
type tokenType int

const (
	tStart   tokenType = iota - 3 // Indicates start of file. Never emitted, but used for testing.
	tError                        // Produced when an error occurs.
	tEOF                          // Indicates end of file. Always the last token.
	tInvalid                      // Should never be produced.

	tSpace         // isSpace*
	tInlineComment // rInlineComment ... \n
	tBlockComment  // rBlockComment ... rBlockCommentEnd
	tAnnotation    // rAnnotation ... rAnnotationEnd
	tIdent         // isLetter isIdent*
	tNull          // rNull
	tTrue          // rTrue
	tFalse         // rFalse
	tInf           // rInf
	tNaN           // rNan
	tPos           // rPos
	tNeg           // rNeg
	tInteger       // isDigit+
	tFloat         // isDigit+ rDecimal isDigit+
	tString        // rString ... rString
	tBlob          // rBlob
	tByte          // isHex isHex
	tSep           // rSep
	tAssoc         // rAssoc
	tArrayOpen     // rArrayOpen
	tArrayClose    // rArrayClose
	tMapOpen       // rMapOpen
	tMapClose      // rMapClose
	tStructOpen    // rStructOpen
	tStructClose   // rStructClose
)

// Returns a string representation of the token.
func (t tokenType) String() string {
	switch t {
	case tError:
		return "Error"
	case tEOF:
		return "EOF"
	default:
		return "Invalid"
	case tSpace:
		return "Space"
	case tInlineComment:
		return "InlineComment"
	case tBlockComment:
		return "BlockComment"
	case tAnnotation:
		return "Annotation"
	case tIdent:
		return "Ident"
	case tNull:
		return "Null"
	case tTrue:
		return "True"
	case tFalse:
		return "False"
	case tInf:
		return "Inf"
	case tNaN:
		return "NaN"
	case tPos:
		return "Pos"
	case tNeg:
		return "Neg"
	case tInteger:
		return "Integer"
	case tFloat:
		return "Float"
	case tString:
		return "String"
	case tBlob:
		return "Blob"
	case tByte:
		return "Byte"
	case tSep:
		return "Sep"
	case tAssoc:
		return "Assoc"
	case tArrayOpen:
		return "ArrayOpen"
	case tArrayClose:
		return "ArrayClose"
	case tMapOpen:
		return "MapOpen"
	case tMapClose:
		return "MapClose"
	case tStructOpen:
		return "StructOpen"
	case tStructClose:
		return "StructClose"
	}
}

// Contains information about the position of a token.
type position struct {
	StartOffset int64
	StartLine   int
	StartColumn int

	EndOffset int64
	EndLine   int
	EndColumn int
}

// Formats the position as a line and column.
func (p position) String() string {
	if p.StartLine == p.EndLine && p.StartColumn == p.EndColumn {
		return fmt.Sprintf("%d:%d", p.StartLine, p.StartColumn)
	}
	return fmt.Sprintf("%d:%d-%d:%d",
		p.StartLine, p.StartColumn,
		p.EndLine, p.EndColumn,
	)
}

// Formats the position as a byte offset.
func (p position) StringOffset() string {
	if p.StartOffset == p.EndOffset {
		return fmt.Sprintf("%d", p.StartOffset)
	}
	return fmt.Sprintf("%d-%d", p.StartOffset, p.EndOffset)
}

// A token emitted from the lexer.
type token struct {
	Type     tokenType
	Position position
	Value    string
	Err      error
}

func mapPrintable(r rune) rune {
	switch {
	case unicode.IsSpace(r):
		switch r {
		case '\n':
			return 'n'
		case '\t':
			return 't'
		case ' ':
			return 's'
		default:
			return '.'
		}
	case unicode.IsPrint(r):
		return r
	default:
		return '.'
	}
}

// Returns a readable representation of the token.
func (t token) String() string {
	return fmt.Sprintf("%s: %s", t.Type, t.StringValue())
}

func (t token) StringValue() string {
	var v string
	if t.Err == nil {
		v = t.Value
		v = strings.Map(mapPrintable, v)
	} else {
		v = t.Err.Error()
	}
	return v
}

// Interprets a token as an error.
type tokenError token

// Formats as the position of the token, then the underlying error.
func (t tokenError) Error() string {
	if t.Err == nil {
		return fmt.Sprintf("%s: %s", t.Position, "no error")
	}
	return fmt.Sprintf("%s: %s", t.Position, t.Err.Error())
}

func (t tokenError) Unwrap() error {
	return t.Err
}

// In which state the lexer is running.
type state func(l *lexer) state

// Emits tokens decoded from a Reader.
type lexer struct {
	lr     *parse.LineReader
	r      *parse.TextReader // Input to read from. Also contains the token buffer.
	start  int64             // Offset of start of buffer.
	tokens chan token        // Where tokens are emitted.
	token  token             // The last token received.

	// Determines the next state to enter for states that have indefinite paths.
	// Enables nested values.
	stack []state
}

// Returns a new lexer that decodes from r.
func newLexer(r io.Reader) *lexer {
	lr := parse.NewLineReader(r)
	l := &lexer{
		lr:     lr,
		r:      parse.NewTextReader(lr),
		tokens: make(chan token),
	}
	go l.run()
	return l
}

// Next prepares the next token. Returns whether the token was successfully
// received.
func (l *lexer) Next() (ok bool) {
	token, ok := <-l.tokens
	if ok {
		l.token = token
	}
	return ok
}

// Token returns the last token emitted.
func (l *lexer) Token() token {
	return l.token
}

// If the last token is an error token, returns its error.
func (l *lexer) Err() error {
	if l.token.Err == nil {
		return nil
	}
	return tokenError(l.token)
}

// Runs the lexer through each state, starting with lexMain, and ending with
// lexEOF.
func (l *lexer) run() {
	l.stack = l.stack[:0]
	for state := lexMain; state != nil; {
		// name := runtime.FuncForPC(reflect.ValueOf(state).Pointer()).Name()
		// name = strings.TrimPrefix(name, "github.com/anaminus/rod/go.")
		// fmt.Println("STATE", name)
		state = state(l)
	}
	close(l.tokens)
}

// Pushes each state onto the stack such that they run in argument order.
func (l *lexer) push(s ...state) {
	for i := len(s) - 1; i >= 0; i-- {
		l.stack = append(l.stack, s[i])
	}
}

// Pops a state from the stack.
func (l *lexer) pop() state {
	n := len(l.stack) - 1
	if n < 0 {
		//TODO: Should probably panic instead.
		return nil
	}
	s := l.stack[n]
	l.stack[n] = nil
	l.stack = l.stack[:n]
	return s
}

// Pushes each state onto the stack such that they run in argument order, then
// immediately pops the stack.
func (l *lexer) do(s ...state) state {
	next := s[0]
	s = s[1:]
	for i := len(s) - 1; i >= 0; i-- {
		l.stack = append(l.stack, s[i])
	}
	return next
}

// Returns the current position of the buffer.
func (l *lexer) position() position {
	p := position{
		StartOffset: l.start,
		EndOffset:   l.r.N(),
	}
	p.StartLine, p.StartColumn = l.lr.Position(p.StartOffset)
	p.EndLine, p.EndColumn = l.lr.Position(p.EndOffset)
	return p
}

// Consumes buffer, returning a string.
func (l *lexer) consume() string {
	l.start = l.r.N()
	return string(l.r.Consume())
}

// Returns the current buffer as a string.
func (l *lexer) bytes() string {
	return string(l.r.Bytes())
}

// Consumes the buffer to emit a token of type t.
func (l *lexer) emit(t tokenType) {
	l.tokens <- token{Type: t, Position: l.position(), Value: string(l.consume())}
}

// Returns whether the buffer is empty.
func (l *lexer) empty() bool {
	return l.r.N() == l.start
}

// Indicates an error produced while lexing.
type lexerError struct {
	Type string
	Err  error
}

// Formats as the error type, then the underlying error.
func (err lexerError) Error() string {
	return fmt.Sprintf("%s error: %s", err.Type, err.Err)
}

// Returns the underlying error.
func (err lexerError) Unwrap() error {
	return err.Err
}

// Emits an error token with an error according to the given format. Returns
// nil, halting the lexer.
func (l *lexer) error(typ string, err error) state {
	err = lexerError{Type: typ, Err: err}
	l.tokens <- token{Type: tError, Position: l.position(), Err: err}
	l.consume()
	return nil
}

type expectedError struct {
	Expected string
	Got      string
}

func (err expectedError) Error() string {
	return fmt.Sprintf("expected %s, got %s", err.Expected, err.Got)
}

// Emits an error token with error that expects a particular value formatted
// according to the given format. Includes the current buffer, or the next
// character if the buffer is empty.
//
// If the underlying reader produces an error, it is displayed instead.
func (l *lexer) expected(format string, a ...any) state {
	if err := l.r.Err(); err != nil && err != io.EOF {
		return l.error("reader", fmt.Errorf("%w", err))
	}
	s := l.bytes()
	if s == "" {
		// Try next character.
		switch r := l.r.MustNext(); {
		case r < 0:
			s = "end of file"
			goto finish
		default:
			s = string(r)
		}
	}
	switch {
	case s == "'":
		// Format with backquotes.
		fallthrough
	default:
		s = fmt.Sprintf("%#q", s)
	case utf8.RuneCountInString(s) == 1:
		// Format as single rune.
		s = fmt.Sprintf("%q", []rune(s)[0])
	}
finish:
	return l.error("syntax", expectedError{
		Expected: fmt.Sprintf(format, a...),
		Got:      fmt.Sprintf("%s", s),
	})
}

////////////////////////////////////////////////////////////////////////////////

// Scans for optional whitespace and comments.
func lexSpace(l *lexer) state {
	l.r.IsAny(unicode.IsSpace)
	if !l.empty() {
		l.emit(tSpace)
	}

	switch {
	case l.r.Is(rBlockComment):
		if !l.r.Until(rBlockCommentEnd) {
			return l.expected("%q", rBlockCommentEnd)
		}
		l.emit(tBlockComment)
		return lexSpace
	case l.r.IsRune(rInlineComment):
		if !l.r.UntilEOL() {
			return l.expected("end of line")
		}
		l.emit(tInlineComment)
		return lexSpace
	}
	return l.pop()
}

// Verifies that the lexer is at the end of the file.
func lexEOF(l *lexer) state {
	if !l.r.IsEOF() {
		return l.expected("end of file")
	}
	l.emit(tEOF)
	return nil
}

// Main entrypoint. Scans any one value.
func lexMain(l *lexer) state {
	if l.r.IsEOF() {
		return l.expected("value")
	}
	return l.do(
		lexSpace, lexAnnotation,
		lexSpace, lexValue,
		lexSpace, lexEOF,
	)
}

// Scan for an optional annotation.
func lexAnnotation(l *lexer) state {
	if l.r.IsRune(rAnnotation) {
		if !l.r.Until(rAnnotationEnd) {
			return l.expected("%q", rAnnotationEnd)
		}
		l.emit(tAnnotation)
	}
	return l.pop()
}

// Tries scanning for a primitive, then tries a composite.
func lexValue(l *lexer) state {
	switch {
	case switchPrimitive(l):
		return l.pop()
	case l.r.IsRune(rArrayOpen):
		l.emit(tArrayOpen)
		return l.do(lexSpace, lexElement)
	case l.r.IsRune(rMapOpen):
		l.emit(tMapOpen)
		return l.do(lexSpace, lexEntry)
	case l.r.IsRune(rStructOpen):
		l.emit(tStructOpen)
		return l.do(lexSpace, lexField)
	default:
		return l.expected("value")
	}
}

// Scans for a primitive.
func lexPrimitive(l *lexer) state {
	switch {
	case switchPrimitive(l):
		return l.pop()
	default:
		return l.expected("primitive value")
	}
}

// Used as a switch case to scan for an optional primitive.
func switchPrimitive(l *lexer) bool {
	switch {
	case l.r.IsRune(rPos):
		l.emit(tPos)
		l.push(lexNumber)
		return true
	case l.r.IsRune(rNeg):
		l.emit(tNeg)
		l.push(lexNumber)
		return true
	case l.r.IsRune(rString):
		l.push(lexString)
		return true
	case l.r.IsRune(rBlob):
		l.emit(tBlob)
		l.push(lexSpace, lexBlob)
		return true
	case isDigit(l.r.Peek()):
		l.push(lexNumber)
		return true
	case l.r.Is(rNull):
		l.emit(tNull)
		return true
	case l.r.Is(rTrue):
		l.emit(tTrue)
		return true
	case l.r.Is(rFalse):
		l.emit(tFalse)
		return true
	case l.r.Is(rInf):
		l.emit(tInf)
		return true
	case l.r.Is(rNaN):
		l.emit(tNaN)
		return true
	default:
		return false
	}
}

// Scans an integer or a float.
func lexNumber(l *lexer) state {
	if l.r.Is(rInf) {
		l.emit(tInf)
		return l.pop()
	}
	l.r.IsAny(isDigit)
	if l.empty() {
		return l.expected("digit")
	}
	if l.r.IsRune(rDecimal) {
		l.r.IsAny(isDigit)
		if l.empty() {
			return l.expected("digit")
		}
		l.emit(tFloat)
		return l.pop()
	}
	l.emit(tInteger)
	return l.pop()
}

// Scans the rest of a string.
func lexString(l *lexer) state {
	for {
		switch l.r.MustNext() {
		case rEscape:
			l.r.MustNext()
		case rString:
			l.emit(tString)
			return l.pop()
		case -1:
			return l.expected("%q", rString)
		}
	}
}

// Scans the rest of a blob.
func lexBlob(l *lexer) state {
	switch r := l.r.MustNext(); {
	case isHex(r):
		if r = l.r.MustNext(); !isHex(r) {
			return l.expected("hexdecimal digit")
		}
		l.emit(tByte)
		return l.do(lexSpace, lexBlob)
	case r == rBlob:
		l.emit(tBlob)
		return l.pop()
	default:
		return l.expected("byte or %q", rBlob)
	}
}

// Scans the element of an array.
func lexElement(l *lexer) state {
	if l.r.IsRune(rArrayClose) {
		l.emit(tArrayClose)
		return l.pop()
	}
	return l.do(
		lexSpace, lexAnnotation,
		lexSpace, lexValue,
		lexSpace, lexElementNext,
	)
}

// Scans the portion following an array element.
func lexElementNext(l *lexer) state {
	switch l.r.MustNext() {
	case rSep:
		l.emit(tSep)
		return l.do(lexSpace, lexElement)
	case rArrayClose:
		l.emit(tArrayClose)
		return l.pop()
	default:
		return l.expected("%q or %q", rSep, rArrayClose)
	}
}

// Scans a map entry.
func lexEntry(l *lexer) state {
	if l.r.IsEOF() {
		return l.expected("map entry key")
	}
	if l.r.IsRune(rMapClose) {
		l.emit(tMapClose)
		return l.pop()
	}
	return l.do(
		lexSpace, lexAnnotation,
		lexSpace, lexPrimitive,
		lexSpace, lexAssoc,
		lexSpace, lexAnnotation,
		lexSpace, lexValue,
		lexSpace, lexEntryNext,
	)
}

// Scans the portion following a map entry.
func lexEntryNext(l *lexer) state {
	switch l.r.MustNext() {
	case rSep:
		l.emit(tSep)
		return l.do(lexSpace, lexEntry)
	case rMapClose:
		l.emit(tMapClose)
		return l.pop()
	default:
		return l.expected("%q or %q", rSep, rMapClose)
	}
}

//x| ( : d )
//x| ( , c : d )
//x| ( b , c : d )
//x| ( : b , c : d )
// | ( a : b , c : d )
//x| ( a : b , c : )
//x| ( a : b , c )
//x| ( a : b c )
// | ( a : b , )
// | ( a : b )
//x| ( a : , )
//x| ( a : )
//x| ( a )
// | ( )

// Scans a struct field.
func lexField(l *lexer) state {
	if l.r.IsRune(rStructClose) {
		l.emit(tStructClose)
		return l.pop()
	}
	if l.r.IsEOF() {
		return l.expected("struct field name")
	}
	return l.do(
		lexIdent,
		lexSpace, lexAssoc,
		lexSpace, lexAnnotation,
		lexSpace, lexValue,
		lexSpace, lexFieldNext,
	)
}

// Scans an identifier.
func lexIdent(l *lexer) state {
	if !isLetter(l.r.MustNext()) {
		return l.expected("identifier")
	}
	if !l.r.IsAny(isIdent) {
		if l.empty() {
			return l.expected("identifier")
		}
	}
	l.emit(tIdent)
	return l.pop()
}

// Scans the portion following a struct field.
func lexFieldNext(l *lexer) state {
	switch l.r.MustNext() {
	case rSep:
		l.emit(tSep)
		return l.do(lexSpace, lexField)
	case rStructClose:
		l.emit(tStructClose)
		return l.pop()
	default:
		return l.expected("%q or %q", rSep, rStructClose)
	}
}

// Scans an association token.
func lexAssoc(l *lexer) state {
	if !l.r.IsRune(rAssoc) {
		return l.expected("%q", rAssoc)
	}
	l.emit(tAssoc)
	return l.pop()
}
