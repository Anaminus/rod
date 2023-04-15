package rod

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
)

type _float = float64
type _int = int64
type _blob = []byte
type _array = []any
type _map = map[any]any
type _struct = map[string]any

var testSpaces = map[string]result{
	"":          {nil, nil},
	" ":         {nil, nil},
	"\n":        {nil, nil},
	"#inline\n": {nil, nil},
	"#<block>":  {nil, nil},
}

var testIdentifiers = map[string]result{
	"Ident": {nil, nil},
	"id0":   {nil, nil},
	"_":     {nil, nil},
}

type result struct {
	v   any
	err error
}

// Map a primitive value string to whether it produces no errors.
var testPrimitives = map[string]result{
	"\"new\nline\"":       {"new\nline", nil},
	`" "`:                 {" ", nil},
	`"back\\slash"`:       {"back\\slash", nil},
	`"Hello, \"world\"!"`: {"Hello, \"world\"!", nil},
	`"Hello, world!"`:     {"Hello, world!", nil},
	`+0.0`:                {_float(0.0), nil},
	`+0`:                  {_int(0), nil},
	`+1234.5678`:          {_float(1234.5678), nil},
	`+12345678`:           {_int(12345678), nil},
	`+1e3`:                {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "end of file", Got: "'e'"}}},
	`+e3`:                 {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "digit", Got: "'e'"}}},
	`+inf`:                {math.Inf(1), nil},
	`+nan`:                {math.NaN(), lexerError{Type: "syntax", Err: expectedError{Expected: "digit", Got: "'n'"}}},
	`-0.0`:                {_float(-0.0), nil},
	`-0`:                  {_int(-0), nil},
	`-1234.5678`:          {_float(-1234.5678), nil},
	`-12345678`:           {_int(-12345678), nil},
	`-1e3`:                {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "end of file", Got: "'e'"}}},
	`-inf`:                {math.Inf(-1), nil},
	`-nan`:                {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "digit", Got: "'n'"}}},
	`0.0`:                 {_float(0.0), nil},
	`0`:                   {_int(0), nil},
	`1234.5678`:           {_float(1234.5678), nil},
	`12345678`:            {_int(12345678), nil},
	`1e3`:                 {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "end of file", Got: "'e'"}}},
	`<anno> "tation"`:     {"tation", nil},
	`false`:               {false, nil},
	`foo`:                 {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "value", Got: "'f'"}}},
	`inf`:                 {math.Inf(1), nil},
	`nan`:                 {math.NaN(), nil},
	`null`:                {nil, nil},
	`true`:                {true, nil},
	`truely`:              {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "end of file", Got: "'l'"}}},
	`| 7F |`:              {_blob{0x7F}, nil},
	`| 12 34 |`:           {_blob{0x12, 0x34}, nil},
	`| | | |`:             {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "end of file", Got: "'|'"}}},
	`| 80 | | FF |`:       {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "end of file", Got: "'|'"}}},
	`| |`:                 {_blob{}, nil},
	`|X0|`:                {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "byte or '|'", Got: "'X'"}}},
	`|0X|`:                {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "hexdecimal digit", Got: "`0X`"}}},
}

var errTODO = errors.New("TODO")

// Map a composite value string to whether it produces an error.
//
// Certain tokens within each string are replaced to produces more combinations.
//
//     "V"   : Replaced by a primitive or composite.
//     "K"   : Replaced by a primitive.
//     I     : Replaced by an identifier.
//     space : Replace by spaces.
//
var testComposites = map[string]result{
	`[`:             {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "element or ']'", Got: "end of file"}}},
	`[ ]`:           {_array{}, nil},
	`[ "V"`:         {nil, lexerError{Type: "reader", Err: io.ErrUnexpectedEOF}},
	`[ "V" ]`:       {_array{"V"}, nil},
	`[ "V" ,`:       {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "element or ']'", Got: "end of file"}}},
	`[ "V" , ]`:     {_array{"V"}, nil},
	`[ "V" , "X"`:   {nil, lexerError{Type: "reader", Err: io.ErrUnexpectedEOF}},
	`[ "V" , "X"]`:  {_array{"V", "X"}, nil},
	`[ "V" , "X",]`: {_array{"V", "X"}, nil},
	`(`:             {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "entry or ')'", Got: "end of file"}}},
	`( )`:           {_map{}, nil},
	`( "K"`:         {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "':'", Got: "end of file"}}},
	`( "K" )`:       {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "':'", Got: "')'"}}},
	`( "K" :`:       {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "value", Got: "end of file"}}},
	`( "K" : )`:     {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "value", Got: "')'"}}},
	`( "K" : "V"`:   {nil, lexerError{Type: "reader", Err: io.ErrUnexpectedEOF}},
	// `( "K" : "V"`:            {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "',' or ')'", Got: "end of file"}}},
	`( "K" : "V" )`:        {_map{"K": "V"}, nil},
	`( "K" : "V" ,`:        {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "entry or ')'", Got: "end of file"}}},
	`( "K" : "V" , )`:      {_map{"K": "V"}, nil},
	`( "K" : "V" ,"X"`:     {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "':'", Got: "end of file"}}},
	`( "K" : "V" ,"X")`:    {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "':'", Got: "')'"}}},
	`( "K" : "V" ,"X":`:    {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "value", Got: "end of file"}}},
	`( "K" : "V" ,"X":"X"`: {nil, lexerError{Type: "reader", Err: io.ErrUnexpectedEOF}},
	// `( "K" : "V" ,"X":"X"`:   {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "',' or ')'", Got: "end of file"}}},
	`( "K" : "V" ,"X":"X")`:  {_map{"K": "V", "X": "X"}, nil},
	`( "K" : "V" ,"X":"X",`:  {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "entry or ')'", Got: "end of file"}}},
	`( "K" : "V" ,"X":"X",)`: {_map{"K": "V", "X": "X"}, nil},
	`{`:                      {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "field or '}'", Got: "end of file"}}},
	`{ }`:                    {_struct{}, nil},
	`{ I`:                    {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "':'", Got: "end of file"}}},
	`{ I }`:                  {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "':'", Got: "'}'"}}},
	`{ I :`:                  {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "value", Got: "end of file"}}},
	`{ I : }`:                {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "value", Got: "'}'"}}},
	`{ I : "V"`:              {nil, lexerError{Type: "reader", Err: io.ErrUnexpectedEOF}},
	// `{ I : "V"`:              {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "',' or '}'", Got: "end of file"}}},
	`{ I : "V" }`:        {_struct{"I": "V"}, nil},
	`{ I : "V" ,`:        {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "field or '}'", Got: "end of file"}}},
	`{ I : "V" , }`:      {_struct{"I": "V"}, nil},
	`{ I : "V" ,"X"`:     {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "identifier", Got: "'\"'"}}},
	`{ I : "V" ,X`:       {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "':'", Got: "end of file"}}},
	`{ I : "V" ,X}`:      {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "':'", Got: "'}'"}}},
	`{ I : "V" ,X:`:      {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "value", Got: "end of file"}}},
	`{ I : "V" ,X:"X"`:   {nil, lexerError{Type: "reader", Err: io.ErrUnexpectedEOF}},
	`{ I : "V" ,X:"X"}`:  {_struct{"I": "V", "X": "X"}, nil},
	`{ I : "V" ,X:"X",`:  {nil, lexerError{Type: "syntax", Err: expectedError{Expected: "field or '}'", Got: "end of file"}}},
	`{ I : "V" ,X:"X",}`: {_struct{"I": "V", "X": "X"}, nil},
}

var testExtra = map[string]error{
	``: lexerError{Type: "syntax", Err: expectedError{Expected: "value", Got: "end of file"}},
}

func keysOf[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writePrimitive(w io.Writer, s string, e error) {
	fmt.Fprintf(w, "\n```%s```\n", s)
	l := newLexer(strings.NewReader(s))
	for l.Next() {
		tok := l.Token()
		v := fmt.Sprintf("%#q", tok.Value)
		if tok.Err != nil {
			v = tok.Err.Error()
		}
		fmt.Fprintf(w, "%03d %-13s: %s\n", tok.Position.StartOffset, tok.Type, v)
	}

	err := l.Err()

	if e == nil && err != nil {
		fmt.Fprintf(w, "FAIL: unexpected error\n")
		return
	}

	if e == nil {
		fmt.Fprintf(w, "okay: got no error\n")
		return
	}

	// e != ""
	if err == nil {
		fmt.Fprintf(w, "FAIL: expected error\n")
		return
	}

	// err != nil
	if err.(tokenError).Err != e {
		fmt.Fprintf(w, "FAIL: expected   : %s\n", e)
		return
	}

	fmt.Fprintf(w, "okay: got expected error\n")
	return
}

func pow(v, p int) int {
	r := 1
	for i := 0; i < p; i++ {
		r *= v
	}
	return r
}

// Traverse each base split by split, with each permutation of perm.
//
//     split = " "
//     base  = "A B"
//     perm  = ["0", "1"]
//
//     Result:
//     0A0B0
//     1A0B0
//     0A1B0
//     1A1B0
//     0A0B1
//     1A0B1
//     0A1B1
//     1A1B1
//
func permutate(w io.Writer, split string, base, perm map[string]error) {
	bases := keysOf(base)
	perms := keysOf(perm)
	blen := len(bases)
	for b := 0; b < blen; b++ {
		s := bases[b]
		r := testPrimitives[s]
		ss := strings.Split(s, split)
		q := pow(len(perm), (len(ss) + 1))
		for v := 0; v < q; v++ {
			var result strings.Builder
			result.WriteString(perms[v/1%len(perms)])
			for i, t := range ss {
				result.WriteString(t)
				result.WriteString(perms[v/pow(len(perms), i+1)%len(perms)])
			}
			res := result.String()
			writePrimitive(w, res, r.err)
		}
	}
}

// For each base, at each split, traverse perm.
//
//     split = " "
//     base  = "A B"
//     perm  = ["0", "1"]
//
//     Result:
//     0AB
//     1AB
//     A0B
//     A1B
//     AB0
//     AB1
//
func traverseEach(w io.Writer, split string, base, perm map[string]result) {
	bases := keysOf(base)
	perms := keysOf(perm)
	for _, b := range bases {
		ss := strings.Split(b, split)
		for i := 0; i < len(ss)+1; i++ {
			for _, p := range perms {
				var result strings.Builder
				for j := 0; j < i; j++ {
					result.WriteString(ss[j])
				}
				result.WriteString(p)
				for j := i; j < len(ss); j++ {
					result.WriteString(ss[j])
				}
				writePrimitive(w, result.String(), base[b].err)
			}
		}
	}
}

func TestGenerate(t *testing.T) {
	f, _ := os.Create("testdata/generated.snapshot")
	defer f.Close()
	w := bufio.NewWriter(f)

	traverseEach(w, " ", testPrimitives, testSpaces)
	traverseEach(w, " ", testComposites, testSpaces)

	for _, extra := range keysOf(testExtra) {
		e := testExtra[extra]
		writePrimitive(w, extra, e)
	}

	w.Flush()
}

func TestLexer(t *testing.T) {
	b, err := os.ReadFile("testdata/sample.rod")
	if err != nil {
		t.Fatalf("%s", err)
		return
	}

	l := newLexer(bytes.NewReader(b))
	for l.Next() {
		// fmt.Println("TOKEN", l.Token().Position.StartOffset, l.Token())
	}
	if err := l.Err(); err != nil {
		t.Error(err)
	}
}

func testFuzz(t *testing.T, a string) {
	r := strings.NewReader(a)
	l := newLexer(r)
	prev := tStart
	for l.Next() {
		token := l.Token()
		next := token.Type
		valid := matrix[[2]tokenType{prev, next}]
		if !valid {
			t.Errorf("lexer produced unexpected token sequence %s -> %s", prev, next)
		}
		prev = next
	}
	if err := l.Err(); err != nil {
		t.Log("valid error:", err)
		return
	}
	switch prev {
	case tEOF:
	default:
		t.Errorf("lexer produced unexpected final token %s", prev)
	}
}

func FuzzLexer(f *testing.F) {
	cases := []string{
		``,
		`null`,
		`true`,
		`false`,
		`inf`,
		`nan`,
		`+42`,
		`-3.141592653589793`,
		`"Hello, world!"`,
		`# Comment`,
		`#<Comment>`,
		`|
			53 74 72 61 6e 67 65 20  67 61 6d 65 2e 0a 54 68
			65 20 6f 6e 6c 79 20 77  69 6e 6e 69 6e 67 20 6d
			6f 76 65 0a 69 73 20 6e  6f 74 20 74 6f 20 70 6c
			61 79 2e
		|`,
		`[1,2,3]`,
		`("A": 1, "B": 2, "C": 3)`,
		`{A: 1, B: 2, C: 3}`,
	}
	for _, c := range cases {
		f.Add(c)
	}
	f.Fuzz(testFuzz)
}
