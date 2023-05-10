# Readable Object Description
A readable format for describing data.

# Goals
- **Readable by humans.**
- **Decodability.**
- **Encodability.**
- **Comparability.** Files in format can be easily compared semantically.
- **Single representation.** Tokens of the syntax are unambiguous.

**Non-goals**
- Writable by humans.

# Overview
A ROD-formatted file contains a value of one of the following types:

Type              | Example                                        | Description
------------------|------------------------------------------------|------------
[null](#null)     | `null`                                         | The absence of a value.
[bool](#bool)     | `true`                                         | A boolean value.
[int](#int)       | `42`                                           | A numeric integer.
[float](#float)   | `-3.14159`                                     | A floating point number.
[string](#string) | `"Hello, world!"`                              | A sequence of characters.
[blob](#blob)     | `\| 48 65 6C 6C 6F 2C 20 77 6F 72 6C 64 21 \|` | A sequence of bytes.
[array](#array)   | `[true, 42, "foo"]`                            | A sequence of values.
[map](#map)       | `(0: "A", true: "B", null: "C")`               | A collection of values mapped to values.
[struct](#struct) | `{X: -2.3, Y: 0.0, Z: 1.9}`                    | A collection of identifiers mapped to values.

# Syntax
The grammar notation used throughout this document is similar to Extended
Backus-Naur Form (EBNF):

```
(in decreasing precedence)

#...        Comment/directive
main = A    Use A as the entrypoint
T = A       Assign expression A to term T
`A`         Literal "A"
A - B       A except B
A B         A followed by B
{ A }       0 or more A
[ A ]       0 or 1 A
( A )       Group of A
A | B       A or B
```

## Characters
ROD files are encoded in UTF-8. The following terms denote Unicode characters:

```
newline = # the Unicode code point U+000A
all     = # any Unicode code point except newline
letter  = # any Unicode code point in category "Letter"
space   = # any unicode code point in category "Separator, space"
```

## Numeric values
Numeric values are composed of sequences of one or more digits:

```
digit  = `0` | `1` | `2` | `3` | `4` | `5` | `6` | `7` | `8` | `9`
digits = digit { digit }
```

Numeric values may also be positive or negative, which is denoted by a sign:

```
sign = `-` | `+`
```

## Whitespace
Whitespace in a ROD file is not significant, and completely optional.

```
inline  = `#` { all - newline } newline
block   = `#<` { all - `>` } `>`
comment = inline | block
_       = { space } { comment { space } }
```

Comments are possible within whitespace. An inline comment appears one line:

```
# An inline comment
```

A block comment can span multiple lines:

```
#<
A
block
comment
>
```

## Types
A ROD file consists of a single value of a specific type.

```
annotation = `<` { all - `>` } `>`
literal    = primitive | composite
value      = [ annotation _ ] literal

main = _ value _
```

Any value may be prefixed with an optional "annotation", delimited by angle
brackets:

```
# Useful for hinting types to the client.
<float32> 3.14

# Can contain anything except a closing angle bracket.
<an annotation with arbitrary text applied to a boolean value> false
```

Types are divided into two categories: *primitives* and *composites*.

```
primitive = null | bool | int | float | string | blob
composite = array | map | struct
```

A primitive forms a single unit, while a composite is composed of a number of
other values.

### null
A null value represents the absence of a value, denoted by the `null` keyword.

```
null = `null`
```

### bool
A bool value represents a Boolean truth value, denoted by the `true` and `false`
keywords.

```
bool = `true` | `false`
```

### int
An int value represents an arbitrary numeric integer. It is denoted by an
optional sign followed by a sequence of digits.

```
int  = [ sign ] digits
```

A `-` sign indicates a negative value, while `+` or no sign indicates a positive
value.

```
-42
 42
+42
```

### float
A float value represents a floating-point number with arbitrary precision. It is
denoted by an optional sign, followed by a sequence of digits, a decimal, then
another sequence of digits.

```
float = [ sign ] (digits `.` digits | `inf`) | `nan`
```

As with the int type, a `-` sign indicates a negative value, while `+` or no
sign indicates a positive value.

The keyword `inf` is used to denote an infinite value. It may have an optional
sign.

The keyword `nan` is used to denote "Not a Number", or an unrepresentable
floating-point value. It cannot have a sign.

```
-3.141592653589793
 3.141592653589793
+3.141592653589793
-inf
 inf
+inf
nan
42.0 # Integer float
```

### string
A string value represents a sequence of characters. It is delimited by
double-quote characters.

```
escape = `\\` | `\"` | `\r` | `\n`
string = `"` { escape | all - `"` } `"`
```

The following characters can be escaped:

Escaped sequence | Character
-----------------|----------
`\\`             | Backslash U+005C, which is otherwise used for escaping.
`\"`             | Double-quote U+0022, which otherwise delimits the string.
`\r`             | Carriage return U+000D, which is otherwise normalized when a part of a CRLF newline.
`\n`             | Line feed U+000A, which is otherwise normalized when a part of a CRLF newline.

A decoder must normalize literal CRLF-style newlines to LF. An encoder must
escape CRLF newlines within a string as `\r\n`, so that a decoder doesn't later
misinterpret them.

```
"Hello, world!"

# Strings may contain newlines.
"Strange game.
The only winning move
is not to play."

# CRLF newlines can be used by escaping the characters.
"Strange game.\r
The only winning move\r
is not to play."

"Strange game.\r\nThe only winning move\r\nis not to play."
```

### blob
A blob value represents a sequence of bytes. It is delimited by pipe characters.

```
hex  = digit | `A` | `B` | `C` | `D` | `E` | `F` | `a` | `b` | `c` | `d` | `e` | `f`
byte = hex hex
blob = `|` _ [ byte { _ byte } _ ] `|`
```

Within the delimiters is a sequence of bytes, each denoted by two hexadecimal
digits.

```
# Compact
|48656C6C6F2C20776F726C6421|

# More readable
| 48 65 6C 6C 6F 2C 20 77 6F 72 6C 64 21 |

# To further aid readability, an encoder may format a blob similar to a hex dump
# by using a combination of whitespace and comments:
|
	53 74 72 61 6e 67 65 20  67 61 6d 65 2e 0a 54 68 #Strange game..Th#
	65 20 6f 6e 6c 79 20 77  69 6e 6e 69 6e 67 20 6d #e only winning m#
	6f 76 65 0a 69 73 20 6e  6f 74 20 74 6f 20 70 6c #ove.is not to pl#
	61 79 2e                                         #ay.#
|
```

### array
An array represents a sequence of values. It is delimited by angle brackets.

```
array = `[` _ [ value { _ `,` _ value } [ _ `,` ] _ ] `]`
```

Each element in the array is a value, separated by a comma. The final element
may have an optional trailing comma to aid with formatting on multiple lines.

```
[1, 2, 3]

[
	1,
	2,
	3,
]
```

### map
A map represents a collection of primitives mapped to values. It is delimited by
parentheses.

```
entry = primitive _ `:` _ value
map   = `(` _ [ entry { _ `,` _ entry } [ _ `,` ] _ ] `)`
```

Each entry in the map is a primitive representing the key, followed by a colon,
then a value representing the value of the entry. Entries are separated by
commas. The final entry may have an optional trailing comma to aid with
formatting on multiple lines.

A map must not have more than one entry with the same key, determined by
equality. For this purpose, NaN is considered equal to NaN. If this occurs, a
decoder must choose to either prefer the latter entry, or emit an error.

Composite values cannot be used as keys.

The entries of a map are unordered. However, an encoder must encode entries in a
consistent order. Keys are ordered according to the following table:

Type   | Index | Value Comparison
-------|------:|-----------------
null   | 1     | Never compared with another null.
bool   | 2     | False before true (`!A && B`).
int    | 3     | Ascending numerically (`A < B`).
float  | 4     | Ascending numerically (`A < B`).
string | 5     | Ascending lexicographically (`A < B`).
blob   | 6     | Ascending lexicographically (`A < B`).

Values of different types are sorted ascending according to their type's Index.
Values of the same type are sorted according to the Value Comparison.

```
("A": 1, "B": 2, "C": 3)

(
	"A": 1,
	"B": 2,
	"C": 3,
)
```

### struct
A struct represents a collection of identifiers mapped to values. It is
delimited by parentheses.

```
ident  = ( letter | `_` ) { digit | letter | `_` }
field  = ident _ `:` _ value
struct = `{` _ [ field { _ `,` _ field } [ _ `,` ] _ ] `}`
```

Each field in the struct is an identifier representing the name, followed by a
colon, then a value representing the value of the field. Fields are separated by
commas. The final field may have an optional trailing comma to aid with
formatting on multiple lines.

An identifier is a sequence of letters, digits, and underscores, which does not
begin with a digit.

A struct must not have more than one field with the same identifier, determined
by equality. If this occurs, a decoder must choose to either prefer the latter
field, or emit an error.

The fields of a struct may be ordered. However, if an encoder does not support
ordered fields, it must sort fields by identifier, ascending lexicographically.

```
{A: 1, B: 2, C: 3}

{
	A: 1,
	B: 2,
	C: 3,
}
```

# Grammar
The complete ROD grammar:

```
newline = # the Unicode code point U+000A
all     = # any Unicode code point except newline
letter  = # any Unicode code point in category "Letter"
space   = # any unicode code point in category "Separator, space"

digit  = `0` | `1` | `2` | `3` | `4` | `5` | `6` | `7` | `8` | `9`
digits = digit { digit }
sign   = `-` | `+`

inline  = `#` { all - newline } newline
block   = `#<` { all - `>` } `>`
comment = inline | block
_       = { space } { comment { space } }

null = `null`
bool = `true` | `false`

int   = [ sign ] digits
float = [ sign ] (digits `.` digits | `inf`) | `nan`

escape = `\\` | `\"` | `\r` | `\n`
string = `"` { escape | all - `"` } `"`

hex  = digit | `A` | `B` | `C` | `D` | `E` | `F` | `a` | `b` | `c` | `d` | `e` | `f`
byte = hex hex
blob = `|` _ [ byte { _ byte } _ ] `|`

array = `[` _ [ value { _ `,` _ value } [ _ `,` ] _ ] `]`

entry = primitive _ `:` _ value
map   = `(` _ [ entry { _ `,` _ entry } [ _ `,` ] _ ] `)`

ident  = ( letter | `_` ) { digit | letter | `_` }
field  = ident _ `:` _ value
struct = `{` _ [ field { _ `,` _ field } [ _ `,` ] _ ] `}`

primitive = null | bool | int | float | string | blob
composite = array | map | struct

annotation = `<` { all - `>` } `>`
literal    = primitive | composite
value      = [ annotation _ ] literal

main = _ value _
```
