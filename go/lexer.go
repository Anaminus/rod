package rod

import (
	"fmt"
	"io"
	"strings"
	"unicode"

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
	tError tokenType = iota - 2
	tEOF
	tInvalid

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

// A token emitted from the lexer.
type token struct {
	Type  tokenType
	Value string
	Error error
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
	if t.Error != nil {
		return fmt.Sprintf("%s: %s", t.Type, t.Error)
	}
	v := t.Value
	v = strings.Map(mapPrintable, v)
	return fmt.Sprintf("%s: %s", t.Type, v)
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
	l.token, ok = <-l.tokens
	return ok
}

// Token returns the last token emitted.
func (l *lexer) Token() token {
	return l.token
}

// If the last token is an error token, returns its error.
func (l *lexer) Err() error {
	return l.token.Error
}

// Runs the lexer through each state, starting with lexMain, and ending with
// lexEOF.
func (l *lexer) run() {
	l.stack = l.stack[:0]
	l.push(lexSpace, lexEOF)
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

// Consumes buffer, returning a string.
func (l *lexer) consume() string {
	l.start = l.r.N()
	return string(l.r.Consume())
}

// Consumes the buffer to emit a token of type t.
func (l *lexer) emit(t tokenType) {
	l.tokens <- token{Type: t, Value: string(l.consume())}
}

// Returns whether the buffer is empty.
func (l *lexer) empty() bool {
	return l.r.N() == l.start
}

// Indicates an error produced while lexing.
type syntaxError struct {
	StartOffset int64
	StartLine   int
	StartColumn int
	EndOffset   int64
	EndLine     int
	EndColumn   int
	Err         error
}

// Implements error.
func (err syntaxError) Error() string {
	return fmt.Sprintf("line %d, column %d: syntax error: %s", err.StartLine, err.StartColumn, err.Err)
}

// Emits an error token with an error according to the given format. Returns
// nil, halting the lexer.
func (l *lexer) errorf(format string, a ...any) state {
	err := syntaxError{
		StartOffset: l.start,
		EndOffset:   l.r.N(),
		Err:         fmt.Errorf(format, a...),
	}
	err.StartLine, err.StartColumn = l.lr.Position(err.StartOffset)
	err.EndLine, err.EndColumn = l.lr.Position(err.EndOffset)
	l.consume()
	l.tokens <- token{Type: tError, Error: err}
	return nil
}

// Causes lexSpace to run, followed by s.
func (l *lexer) lexSpaceThen(s state) state {
	l.push(s)
	return lexSpace
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
			return l.errorf("expected %q", rBlockCommentEnd)
		}
		l.emit(tBlockComment)
		return lexSpace
	case l.r.IsRune(rInlineComment):
		if !l.r.UntilEOL() {
			return l.errorf("expected end of line")
		}
		l.emit(tInlineComment)
		return lexSpace
	}
	return l.pop()
}

// Verifies that the lexer is at the end of the file.
func lexEOF(l *lexer) state {
	if !l.r.IsEOF() {
		return l.errorf("expected end of file")
	}
	l.emit(tEOF)
	return nil
}

// Main entrypoint. Scans any one value.
func lexMain(l *lexer) state {
	return l.lexSpaceThen(lexValue)
}

// Attempt to scan a value annotation, then a primitive.
//
// Not quite a state function. Instead, it pushes a state, then returns true if
// the caller should pop the state, or false if the caller should continue.
// Necessary because lexValue and lexOnlyPrimitive are very similar, diverging
// only after the primitive is scanned, but the decision to use one or the other
// is made before the annotation is scanned.
func pushValue(l *lexer) bool {
	if l.r.IsRune(rAnnotation) {
		if !l.r.Until(rAnnotationEnd) {
			l.push(l.errorf("expected %q", rAnnotationEnd))
			return true
		}
		l.emit(tAnnotation)
	}

	// Attempt to scan a primitive.
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
	}
	return false
}

// Scans for an optional annotation, then tries a primitive.
func lexValue(l *lexer) state {
	if pushValue(l) {
		return l.pop()
	}
	// Try a composite.
	return lexComposite
}

func lexOnlyPrimitive(l *lexer) state {
	if pushValue(l) {
		return l.pop()
	}
	return l.pop()
}

// Scans a composite value.
func lexComposite(l *lexer) state {
	switch {
	case l.r.IsRune(rArrayOpen):
		l.emit(tArrayOpen)
		return l.lexSpaceThen(lexElement)
	case l.r.IsRune(rMapOpen):
		l.emit(tMapOpen)
		return l.lexSpaceThen(lexEntryKey)
	case l.r.IsRune(rStructOpen):
		l.emit(tStructOpen)
		return l.lexSpaceThen(lexFieldName)
	default:
		return l.pop()
	}
}

// Scans an integer or a float.
func lexNumber(l *lexer) state {
	if !l.r.IsAny(isDigit) {
		return l.errorf("expected digit")
	}
	if l.r.IsRune(rDecimal) {
		if !l.r.IsAny(isDigit) {
			return l.errorf("expected digit")
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
			return l.pop()
		case -1:
			return l.errorf("expected %q", rString)
		}
	}
}

// Scans the rest of a blob. Also attempts to scan subsequent blobs.
func lexBlob(l *lexer) state {
	switch r := l.r.MustNext(); {
	case isHex(r):
		if r = l.r.MustNext(); !isHex(r) {
			return l.errorf("expected hexdecimal digit")
		}
		l.emit(tByte)
		return l.lexSpaceThen(lexBlob)
	case r == rBlob:
		l.emit(tBlob)
		return l.lexSpaceThen(lexAnotherBlob)
	default:
		return l.errorf("expected byte or %q", rBlob)
	}
}

// Attempt to scan another blob.
func lexAnotherBlob(l *lexer) state {
	if l.r.IsRune(rBlob) {
		l.emit(tBlob)
		return l.lexSpaceThen(lexBlob)
	}
	return l.pop()
}

// Scans the element of an array.
func lexElement(l *lexer) state {
	if l.r.IsRune(rArrayClose) {
		l.emit(tArrayClose)
		return l.pop()
	}
	return l.do(
		lexSpace, lexValue,
		lexSpace, lexElementNext,
	)
}

// Scans the portion following an array element.
func lexElementNext(l *lexer) state {
	switch l.r.MustNext() {
	case rSep:
		l.emit(tSep)
		return l.lexSpaceThen(lexElement)
	case rArrayClose:
		l.emit(tArrayClose)
		return l.pop()
	default:
		return l.errorf("expected %q or %q", rSep, rArrayClose)
	}
}

// Scans the key of a map entry.
func lexEntryKey(l *lexer) state {
	if l.r.IsRune(rMapClose) {
		l.emit(tArrayClose)
		return l.pop()
	}
	return l.do(
		lexSpace, lexOnlyPrimitive,
		lexSpace, lexAssoc,
		lexSpace, lexEntryValue,
	)
}

// Scans an association token.
func lexAssoc(l *lexer) state {
	if !l.r.IsRune(rAssoc) {
		return l.errorf("expected %q", rAssoc)
	}
	l.emit(tAssoc)
	return l.pop()
}

// Scans the value of a map entry.
func lexEntryValue(l *lexer) state {
	return l.do(
		lexSpace, lexValue,
		lexSpace, lexEntryNext,
	)
}

// Scans the portion following a map entry.
func lexEntryNext(l *lexer) state {
	switch l.r.MustNext() {
	case rSep:
		l.emit(tSep)
		return l.lexSpaceThen(lexEntryKey)
	case rMapClose:
		l.emit(tMapClose)
		return l.pop()
	default:
		return l.errorf("expected %q or %q", rSep, rMapClose)
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

// Scans the name of a struct field.
func lexFieldName(l *lexer) state {
	if l.r.IsRune(rStructClose) {
		l.emit(tStructClose)
		return l.pop()
	}
	return lexIdent
}

// Scans an identifier.
func lexIdent(l *lexer) state {
	if !isLetter(l.r.MustNext()) {
		return l.errorf("expected identifier")
	}
	if !l.r.IsAny(isIdent) {
		if l.empty() {
			return l.errorf("expected identifier")
		}
	}
	l.emit(tIdent)
	return l.do(
		lexSpace, lexAssoc,
		lexSpace, lexFieldValue,
	)
}

// Scans the value of a struct field.
func lexFieldValue(l *lexer) state {
	return l.do(
		lexSpace, lexValue,
		lexSpace, lexFieldNext,
	)
}

// Scans the portion following a struct field.
func lexFieldNext(l *lexer) state {
	switch l.r.MustNext() {
	case rSep:
		l.emit(tSep)
		return l.lexSpaceThen(lexFieldName)
	case rStructClose:
		l.emit(tStructClose)
		return l.pop()
	default:
		return l.errorf("expected %q or %q", rSep, rStructClose)
	}
}
