local Types = require(script.Parent.Types)
-- local Types = require("Types")

local export = {}

--- Decodes *s* according to the ROD format. If *s* contains a syntax error, an
--- error is thrown.
function export.decode(s: string): any
	type state = () -> state?

	local rInlineComment   = '#'
	local rEOL             = '\n'
	local rBlockComment    = "#<"
	local rBlockCommentEnd = '>'
	local rAnnotation      = '<'
	local rAnnotationEnd   = '>'
	local rNull            = "null"
	local rTrue            = "true"
	local rFalse           = "false"
	local rInf             = "inf"
	local rNaN             = "nan"
	local rPos             = '+'
	local rNeg             = '-'
	local rDecimal         = '.'
	local rString          = '"'
	local rEscape          = '\\'
	local rBlob            = '|'
	local rSep             = ','
	local rAssoc           = ':'
	local rArrayOpen       = '['
	local rArrayClose      = ']'
	local rMapOpen         = '('
	local rMapClose        = ')'
	local rStructOpen      = '{'
	local rStructClose     = '}'

	-- Input string has 3 sections: consumed, read, and unread.

	local i = 1 -- Index of first unconsumed byte.
	local j = 1 -- Index of first unread byte.

	-- Stack of pending states. Certain states have no specific state to move
	-- to, so they pop the stack to get the next state. Likewise, some states
	-- push a number of other states, expecting them to execute in a particular
	-- order.
	local states: {state} = {}
	-- Stack of generated values. States may expect certain partial values at
	-- specific locations on the stack. The bottom value is returned by the
	-- decoder.
	local stack: {any} = {}

	-- Pushes each state onto the state stack such that they run in argument
	-- order.
	local function push(...: state)
		for i = select("#", ...), 1, -1 do
			table.insert(states, (select(i, ...)))
		end
	end

	-- Pops a state from the state stack.
	local function pop(): state
		local next = table.remove(states)
		if not next then
			error("unbalanced state stack")
		end
		return next
	end

	-- Pushes each state onto the state stack such that they run in argument
	-- order, then immediately pops the stack.
	local function run(...: state): state
		push(...)
		return pop()
	end

	-- Returns whether the next unread bytes match *match*, making them read if
	-- so.
	local function is(match: string): boolean
		if string.sub(s, j, j+#match-1) == match then
			j += #match
			return true
		end
		return false
	end

	-- Consumes all read bytes.
	local function skip()
		i = j
	end

	-- Returns read bytes without consuming them.
	local function bytes(): string
		return string.sub(s, i, j-1)
	end

	-- Consumes and returns read bytes.
	local function consume(): string
		local r = string.sub(s, i, j-1)
		i = j
		return r
	end

	-- Returns whether decoder is at the end of the input.
	local function isEOF(): boolean
		return j > #s
	end

	-- Returns whether there are any read bytes.
	local function empty(): boolean
		return j == i
	end

	-- Returns whether *pattern** matches the next unread bytes. If found, reads
	-- up to the position of the match.
	local function find(pattern: string): boolean
		local _, k = string.find(s, pattern, j)
		if k then
			j = k + 1
			return true
		end
		return false
	end

	-- Like find, but looks literally without pattern matching.
	local function literal(literal: string): boolean
		local _, k = string.find(s, literal, j, true)
		if k then
			j = k + 1
			return true
		end
		return false
	end

	-- Like find, but does not read the match.
	local function peek(pattern: string): boolean
		return not not string.match(s, pattern, j)
	end

	-- Returns the line and column of *offset*.
	local function position(offset: number): (number, number)
		local r = string.sub(s, 1, offset-1)
		local _, line = string.gsub(r, "\n", "\n")
		local column = string.find(string.reverse(r), "\n")
		return line+1, column or #r+1
	end

	-- Produces an error in which the formatted arguments are expected, but the
	-- read bytes were received. If there are no read bytes, then the next byte
	-- is used instead.
	local function expected(format: string, ...: any): state?
		local got = bytes()
		if got == "" then
			if isEOF() then
				got = "end of file"
			else
				got = string.format("%q", string.sub(j,j))
			end
		else
			got = string.format("%q", got)
		end
		local line, column = position(i)
		local expected = string.format(format, ...)
		local err = string.format("%d:%d: expected %s, got %s", line, column, expected, got)
		--TODO: emit as structure with __tostring.
		error(err, 4) -- level: expected <- state <- decode <- caller
		return nil
	end

	local lexMain
	local lexSpace
	local lexEOF
	local lexAnnotation
	local lexValue
	local lexPrimitive
	local switchPrimitive
	local lexNumber
	local lexString
	local lexBlob
	local lexElement
	local lexElementNext
	local lexEntry
	local lexEntryNext
	local lexField
	local lexIdent
	local lexFieldNext
	local lexAssoc

	-- Main entrypoint. Scans any one value.
	function lexMain(): state?
		if isEOF() then
			return expected("value")
		end
		return run(
			lexSpace, lexAnnotation,
			lexSpace, lexValue,
			lexSpace, lexEOF
		)
	end

	-- Scans for optional whitespace and comments.
	function lexSpace(): state?
		if find("^%s*") then
			skip()
			-- emit Space
		end

		if is(rBlockComment) then
			if not literal(rBlockCommentEnd) then
				return expected("%q", rBlockCommentEnd)
			end
			skip()
			-- emit BlockComment
			return lexSpace
		elseif is(rInlineComment) then
			if not literal(rEOL) and not isEOF() then
				return expected("end of line")
			end
			skip()
			-- emit InlineComment
			return lexSpace
		end
		return pop()
	end

	-- Verifies that the lexer is at the end of the file.
	function lexEOF(): state?
		if not isEOF() then
			return expected("end of file")
		end
		-- emit EOF
		return nil
	end

	-- Scan for an optional annotation.
	function lexAnnotation(): state?
		if is(rAnnotation) then
			if not literal(rAnnotationEnd) then
				return expected("%q", rAnnotationEnd)
			end
			skip()
			-- emit Annotation
		end
		return pop()
	end

	-- Tries scanning for a primitive, then tries a composite.
	function lexValue(): state?
		if switchPrimitive() then
			return pop()
		elseif is(rArrayOpen) then
			skip()
			-- emit ArrayOpen
			table.insert(stack, Types.array())
			return run(lexSpace, lexElement)
		elseif is(rMapOpen) then
			skip()
			-- emit MapOpen
			table.insert(stack, Types.map())
			return run(lexSpace, lexEntry)
		elseif is(rStructOpen) then
			skip()
			-- emit StructOpen
			table.insert(stack, Types.struct())
			return run(lexSpace, lexField)
		else
			return expected("value")
		end
	end

	-- Scans for a primitive.
	function lexPrimitive(): state?
		if switchPrimitive() then
			return pop()
		else
			return expected("primitive value")
		end
	end

	-- Used as a switch case to scan for an optional primitive.
	function switchPrimitive(): boolean
		if is(rPos) then
			skip()
			table.insert(stack, 1)
			push(lexNumber)
			return true
		elseif is(rNeg) then
			skip()
			table.insert(stack, -1)
			push(lexNumber)
			return true
		elseif is(rString) then
			push(lexString)
			return true
		elseif is(rBlob) then
			skip()
			-- emit Blob
			table.insert(stack, {})
			push(lexSpace, lexBlob)
			return true
		elseif peek("^[0-9]") then
			table.insert(stack, 1)
			push(lexNumber)
			return true
		elseif is(rNull) then
			skip()
			-- emit Null
			table.insert(stack, Types.null())
			return true
		elseif is(rTrue) then
			skip()
			-- emit True
			table.insert(stack, Types.bool(true))
			return true
		elseif is(rFalse) then
			skip()
			-- emit False
			table.insert(stack, Types.bool(false))
			return true
		elseif is(rInf) then
			skip()
			-- emit Inf
			table.insert(stack, Types.float(math.huge))
			return true
		elseif is(rNaN) then
			skip()
			-- emit NaN
			table.insert(stack, Types.float(0/0))
			return true
		else
			return false
		end
	end

	-- Scans an integer or a float. Expects top value to be a sign to multiply
	-- by.
	function lexNumber(): state?
		if is(rInf) then
			skip()
			stack[#stack] *= math.huge
			return pop()
		end

		if not find("^[0-9]+") then
			return expected("digit")
		end
		if is(rDecimal) then
			if not find("^[0-9]+") then
				return expected("digit")
			end
			-- emit Float
			stack[#stack] *= tonumber(consume())
			stack[#stack] = Types.float(stack[#stack])
			return pop()
		end
		-- emit Integer
		stack[#stack] *= tonumber(consume())
		stack[#stack] = Types.int(stack[#stack])
		return pop()
	end

	-- Scans the rest of a string.
	function lexString(): state?
		local buf = {}
		while true do
			-- Jump to next escape or delimiter.
			local _, k, prev, sep = string.find(s, "(.-)(["..rEscape..rString.."])", j)
			if sep == "\\" then
				table.insert(buf, prev)
				j = k + 1
				table.insert(buf, string.sub(s, j, j))
				j = j + 1
			elseif sep == "\"" then
				table.insert(buf, prev)
				j = k + 1
				skip()
				-- emit String
				table.insert(stack, table.concat(buf))
				return pop()
			else
				return expected("%q", rString)
			end
		end
	end

	-- Scans the rest of a blob. Expects top value to be a table of strings.
	function lexBlob(): state?
		if find("^%x") then
			if not find("^%x") then
				return expected("hexdecimal digit")
			end
			-- emit Byte
			local byte = tonumber(consume(), 16)
			table.insert(stack[#stack], string.char(byte))
			return run(lexSpace, lexBlob)
		elseif is(rBlob) then
			skip()
			-- emit Blob
			stack[#stack] = table.concat(stack[#stack])
			return pop()
		else
			return expected("byte or %q", rBlob)
		end
	end

	-- Scans the element of an array.
	function lexElement(): state?
		if isEOF() then
			return expected("element or %q", rArrayClose)
		end
		if is(rArrayClose) then
			skip()
			-- emit ArrayClose
			return pop()
		end
		return run(
			lexSpace, lexAnnotation,
			lexSpace, lexValue,
			lexSpace, lexElementNext
		)
	end

	-- Pops an array element off the stack and appends it to an array.
	local function popElement()
		local element = table.remove(stack)
		local array = stack[#stack]
		table.insert(array, element)
	end

	-- Scans the portion following an array element. Expects top value to be
	-- element, and top-1 to be array.
	function lexElementNext(): state?
		if is(rSep) then
			skip()
			-- emit Sep
			popElement()
			return run(lexSpace, lexElement)
		elseif is(rArrayClose) then
			skip()
			-- emit ArrayClose
			popElement()
			return pop()
		else
			return expected("%q or %q", rSep, rArrayClose)
		end
	end

	-- Scans a map entry.
	function lexEntry(): state?
		if isEOF() then
			return expected("entry or %q", rMapClose)
		end
		if is(rMapClose) then
			skip()
			-- emit MapClose
			return pop()
		end
		return run(
			lexSpace, lexAnnotation,
			lexSpace, lexPrimitive,
			lexSpace, lexAssoc,
			lexSpace, lexAnnotation,
			lexSpace, lexValue,
			lexSpace, lexEntryNext
		)
	end

	-- Pops a value and a key from the stack and assigns the key to the value
	-- within a map.
	local function popEntry()
		local value = table.remove(stack)
		local key = table.remove(stack)
		local map = stack[#stack]
		map[key] = value
	end

	-- Scans the portion following a map entry. Expects top value to be entry
	-- value, top-1 to be entry key, and top-2 to be map.
	function lexEntryNext(): state?
		if is(rSep) then
			skip()
			-- emit Sep
			popEntry()
			return run(lexSpace, lexEntry)
		elseif is(rMapClose) then
			skip()
			-- emit MapClose
			popEntry()
			return pop()
		else
			return expected("%q or %q", rSep, rMapClose)
		end
	end

	-- Scans a struct field.
	function lexField(): state?
		if isEOF() then
			return expected("field or %q", rStructClose)
		end
		if is(rStructClose) then
			skip()
			-- emit StructClose
			return pop()
		end
		return run(
			lexIdent,
			lexSpace, lexAssoc,
			lexSpace, lexAnnotation,
			lexSpace, lexValue,
			lexSpace, lexFieldNext
		)
	end

	-- Scans an identifier.
	function lexIdent(): state?
		if not find("^[A-Za-z_]") then
			return expected("identifier")
		end
		if not find("^[0-9A-Za-z_]*") then
			if empty() then
				return expected("identifier")
			end
		end
		-- emit Ident
		table.insert(stack, consume())
		return pop()
	end

	-- Pops a value and identifier from the stack and assigns the identifier to
	-- the value within a struct.
	local function popField()
		local value = table.remove(stack)
		local ident = table.remove(stack)
		local struct = stack[#stack]
		struct[ident] = value
	end

	-- Scans the portion following a struct field.
	function lexFieldNext(): state?
		if is(rSep) then
			skip()
			-- emit Sep
			popField()
			return run(lexSpace, lexField)
		elseif is(rStructClose) then
			skip()
			-- emit StructClose
			popField()
			return pop()
		else
			return expected("%q or %q", rSep, rStructClose)
		end
	end

	-- Scans an association token.
	function lexAssoc(): state?
		if not is(rAssoc) then
			return expected("%q", rAssoc)
		end
		skip()
		-- emit Assoc
		return pop()
	end

	local state = lexMain
	while state do
		state = state()
	end
	return stack[1]
end

return table.freeze(export)
