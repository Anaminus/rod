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

	// Indicates whether composite values are parsed. Set directly before each
	// time lexValue is returned. lexValue is never returned again between the
	// point where comp is set and where it is evaluated, so it's safe to be a
	// single field rather than a stack.
	comp bool
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
	l.push(lexEOF)
	l.push(lexSpace)
	for state := lexMain; state != nil; {
		// name := runtime.FuncForPC(reflect.ValueOf(state).Pointer()).Name()
		// name = strings.TrimPrefix(name, "github.com/anaminus/rod/go.")
		// fmt.Println("STATE", name)
		state = state(l)
	}
	close(l.tokens)
}

// Pushes s onto the stack.
func (l *lexer) push(s state) {
	l.stack = append(l.stack, s)
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

// Consumes buffer, returning a string.
func (l *lexer) consume() string {
	return string(l.r.Consume())
}

// Consumes the buffer to emit a token of type t.
func (l *lexer) emit(t tokenType) {
	l.tokens <- token{Type: t, Value: string(l.consume())}
	l.start = l.r.N()
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
	l.start = l.r.N()
	l.tokens <- token{Type: tError, Error: err}
	return nil
}

// Scans for optional whitespace.
func (l *lexer) whitespace() {
	l.r.IsAny(unicode.IsSpace)
	if !l.empty() {
		l.emit(tSpace)
	}
}

// Scans the rest of a block comment.
func (l *lexer) blockComment() bool {
	if !l.r.Until(rBlockCommentEnd) {
		l.errorf("expected %q", rBlockCommentEnd)
		return false
	}
	l.emit(tBlockComment)
	l.whitespace()
	return true
}

// Scans the rest of an inline comment.
func (l *lexer) inlineComment() bool {
	if !l.r.UntilEOL() {
		l.errorf("expected end of line")
		return false
	}
	l.emit(tInlineComment)
	l.whitespace()
	return true
}

// Scans spacing, which may include a number of comments.
func (l *lexer) space() bool {
	l.whitespace()
	for {
		switch {
		case l.r.Is(rBlockComment):
			return l.blockComment()
		case l.r.IsRune(rInlineComment):
			return l.inlineComment()
		default:
			// Not an error.
			return true
		}
	}
}

// Causes lexSpace to run, followed by s.
func (l *lexer) lexSpaceThen(s state) state {
	l.push(s)
	return lexSpace
}

////////////////////////////////////////////////////////////////////////////////

// Scans for optional whitespace and comments.
func lexSpace(l *lexer) state {
	l.space()
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
	l.comp = true
	return l.lexSpaceThen(lexValue)
}

// Scans for an optional annotation, then tries a primitive.
func lexValue(l *lexer) state {
	if l.r.IsRune(rAnnotation) {
		return lexAnnotation
	}
	return lexPrimitive
}

// Scans the rest of an annotation, then tries a primitive.
func lexAnnotation(l *lexer) state {
	if !l.r.Until(rAnnotationEnd) {
		return l.errorf("expected %q", rAnnotationEnd)
	}
	l.emit(tAnnotation)
	return lexPrimitive
}

// Attempts to scan a primitive value. If one could not be scanned, then
// attempts to scan a composite unless l.comp is false.
func lexPrimitive(l *lexer) state {
	switch {
	case l.r.IsRune(rPos):
		l.emit(tPos)
		return lexNumber
	case l.r.IsRune(rNeg):
		l.emit(tNeg)
		return lexNumber
	case l.r.IsRune(rString):
		return lexString
	case l.r.IsRune(rBlob):
		l.emit(tBlob)
		return l.lexSpaceThen(lexBlob)
	case isDigit(l.r.Peek()):
		return lexNumber
	case l.r.Is(rNull):
		l.emit(tNull)
		return l.pop()
	case l.r.Is(rTrue):
		l.emit(tTrue)
		return l.pop()
	case l.r.Is(rFalse):
		l.emit(tFalse)
		return l.pop()
	case l.r.Is(rInf):
		l.emit(tInf)
		return l.pop()
	case l.r.Is(rNaN):
		l.emit(tNaN)
		return l.pop()
	default:
		if !l.comp {
			return l.pop()
		}
		return lexComposite
	}
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
	l.push(lexElementNext)
	l.push(lexSpace)
	l.comp = true
	return l.lexSpaceThen(lexValue)
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
	l.push(lexEntryAssoc)
	l.push(lexSpace)
	l.comp = false
	return l.lexSpaceThen(lexValue)
}

// Scans the association token a map entry.
func lexEntryAssoc(l *lexer) state {
	if !l.r.IsRune(rAssoc) {
		l.errorf("expected %q", rAssoc)
	}
	l.emit(tAssoc)
	return l.lexSpaceThen(lexEntryValue)
}

// Scans the value of a map entry.
func lexEntryValue(l *lexer) state {
	l.push(lexEntryNext)
	l.push(lexSpace)
	l.comp = true
	return l.lexSpaceThen(lexValue)
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
	return l.lexSpaceThen(lexFieldAssoc)
}

// Scans the association token of a struct field.
func lexFieldAssoc(l *lexer) state {
	if !l.r.IsRune(rAssoc) {
		return l.errorf("expected %q", rAssoc)
	}
	l.emit(tAssoc)
	return l.lexSpaceThen(lexFieldValue)
}

// Scans the value of a struct field.
func lexFieldValue(l *lexer) state {
	l.push(lexFieldNext)
	l.push(lexSpace)
	l.comp = true
	return l.lexSpaceThen(lexValue)
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
