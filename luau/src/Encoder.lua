local c = require(script.Parent.const)
local Types = require(script.Parent.Types)

local export = {}

--- Encodes *v* according to the ROD format. If *v* contains an invalid value,
--- an error is thrown.
function export.encode(value: any): string
	local buf = table.create(256)
	local lead = table.create(8)

	local function push()
		table.insert(lead, c.Indent)
	end

	local function pop()
		table.remove(lead)
	end

	local function write(...)
		for i = 1, select("#", ...) do
			table.insert(buf, (select(i, ...)))
		end
	end

	local function newline()
		write(c.EOL)
		table.move(lead, 1, #lead, #buf+1, buf)
	end

	local encodeValue
	local encodePrimitive
	local encodeNull
	local encodeBool
	local encodeInt
	local encodeFloat
	local encodeString
	local encodeBlob
	local encodeArray
	local encodeMap
	local encodeStruct
	local encodeIdent

	function encodeValue(v: any)
		if encodePrimitive(v) then
			return
		end
		local t = Types.typeof(v)
		if t == "array" then
			encodeArray(v)
			return
		elseif t == "map" then
			encodeMap(v)
			return
		elseif t == "struct" then
			encodeStruct(v)
			return
		else
			error(string.format("cannot encode type %s", typeof(v)))
		end
	end

	function encodePrimitive(v: any): boolean
		local t, v = Types.typeof(v)
		if t == "null" then
			encodeNull()
			return true
		elseif t == "bool" then
			encodeBool(v)
			return true
		elseif t == "int" then
			encodeInt(v)
			return true
		elseif t == "float" then
			encodeFloat(v)
			return true
		elseif t == "string" then
			encodeString(v)
			return true
		elseif t == "blob" then
			encodeBlob(v)
			return true
		end
		return false
	end

	function encodeNull()
		write(c.Null)
	end

	function encodeBool(v: boolean)
		if v then
			write(c.True)
		else
			write(c.False)
		end
	end

	function encodeInt(v: number)
		local int = math.modf(v)
		write(tostring(int))
	end

	-- Convert float to string without scientific notation.
	local function tostringNoE(v: number): string
		local f = tostring(v)
		local e = string.find(f, "e")
		if not e then
			-- No scientific notation.
			if select(2, math.modf(v)) == 0 then
				-- Ensure trailing fractional for float.
				f ..= c.Decimal .. "0"
			end
			return f
		end

		-- Remember sign.
		local neg = false
		if string.match(f, "^%-") then
			f = string.sub(f, 2)
			neg = true
		end

		-- Compute leading or trailing zeros.
		local coef, exp = string.sub(f, 1, e-1), string.sub(f, e+1)
		coef = string.gsub(coef, "%.", "")
		exp = tonumber(exp)
		local result
		if exp > 0 then
			result = coef .. string.rep("0", math.abs(exp-#coef)+1)
		elseif exp < 0 then
			result = "0" .. c.Decimal .. string.rep("0", -exp-1) .. coef
		end

		-- Ensure trailing fractional for float.
		if select(2, math.modf(v)) == 0 then
			result ..= c.Decimal .. "0"
		end

		-- Apply sign.
		if neg then
			result = c.Neg .. result
		end

		return result
	end

	function encodeFloat(v: number)
		if v == math.huge then
			write(c.Inf)
		elseif v == -math.huge then
			write(c.Neg, c.Inf)
		elseif v ~= v then
			write(c.NaN)
		else
			write(tostringNoE(v))
		end
		return nil
	end

	function encodeString(v: string)
		write(c.String)
		write(string.gsub(v, "[" .. c.Escape .. c.String .. "]", function(c)
			return c.Escape .. c
		end))
		write(c.String)
	end

	local function toChar(b: string): string
		if ' ' <= b and b <= '~' then
			return b
		end
		return '.'
	end

	function encodeBlob(v: string)
		write(c.Blob)
		if #v == 0 then
			write(c.Blob)
			return
		end
		push()
		newline()

		local width = 16 -- Bytes per line.
		local half = 8   -- Where to add extra space.
		for i = 0, #v-1 do
			if i%width ~= 0 then
				-- Space before each byte except start of line.
				write(c.Space)
			end
			if (i+half)%width == 0 then
				-- Extra space at half width.
				write(c.Space)
			end
			-- Write byte.
			write(string.format("%02X", string.byte(v, i+1, i+1)))

			-- At end of a full line, display ASCII as comment.
			if (i+1)%width == 0 then
				write(c.Space, c.InlineComment)
				for j = i + 1 - width, i do
					write(toChar(string.sub(v, j+1, j+1)))
				end
				write(c.InlineComment)
				-- If there's more, add a newline.
				if i+1 < #v then
					newline()
				end
			end
		end
		-- Number of extra bytes in last line.
		local n = width - ((#v-1)%width + 1)
		if n > 0 then
			for i = 0, n-1 do
				-- Space for each extra byte.
				write(c.Space, c.Space, c.Space)
			end
			if n >= half then
				-- Extra space at half width.
				write(c.Space)
			end
			write(c.Space, c.InlineComment)
			-- Number of bytes in last line.
			n = #v - (width - n)
			if n < 0 then
				-- Prevent underflow.
				n = 0
			end
			for j = n, #v-1 do
				write(toChar(string.sub(v, j+1, j+1)))
			end
			write(c.InlineComment)
		end

		pop()
		newline()
		write(c.Blob)
	end

	function encodeArray(v: {any})
		write(c.ArrayOpen)
		push()
		for _, v in v do
			newline()
			encodeValue(v)
			write(c.Sep)
		end
		pop()
		if #v > 0 then
			newline()
		end
		write(c.ArrayClose)
	end

	local function typeIndex(v: any): number
		local t = Types.typeof(v)
		if t == "null" then
			return 1
		elseif t == "bool" then
			return 2
		elseif t == "int" then
			return 3
		elseif t == "float" then
			return 4
		elseif t == "string" then
			return 5
		elseif t == "blob" then
			return 6
		else
			return 0
		end
	end

	local function typeCmp(i: any, j: any): boolean
		local t = Types.typeof(i)
		if t == "null" then
			return false
		elseif t == "bool" then
			return not i and j
		elseif t == "int" then
			return i < j
		elseif t == "float" then
			return i < j
		elseif t == "string" then
			return i < j
		elseif t == "blob" then
			return i < j
		else
			return false
		end
	end

	local function mapForEach(m: {[any]: any}, f: (k: any, v: any) -> ())
		local keys = {}
		for key in m do
			table.insert(keys, key)
		end
		table.sort(keys, function(a: any, b: any): boolean
			local ti = typeIndex(a)
			local tj = typeIndex(b)
			if ti == tj then
				return typeCmp(a, b)
			end
			return ti < tj
		end)
		for _, key in keys do
			f(key, m[key])
		end
	end

	function encodeMap(v: {[any]: any})
		write(c.MapOpen)
		push()
		local has = false
		mapForEach(v, function(k: any, v: any)
			has = true
			newline()

			if not encodePrimitive(k) then
				error(string.format("cannot encode type %s as map key", typeof(v)))
			end
			write(c.Assoc, c.Space)
			encodeValue(v)
			write(c.Sep)
			return nil
		end)
		pop()
		if has then
			newline()
		end
		write(c.MapClose)
	end

	local function structForEach(s: {[string]: any}, f: (i: string, v: any) -> ())
		local keys = {}
		for key in s do
			table.insert(keys, key)
		end
		table.sort(keys)
		for _, key in keys do
			f(key, s[key])
		end
	end

	function encodeStruct(v: {[string]: any})
		write(c.StructOpen)
		push()
		local has = false
		structForEach(v, function(i: string, v: any)
			has = true
			newline()
			encodeIdent(i)
			write(c.Assoc, c.Space)
			encodeValue(v)
			write(c.Sep)
		end)
		pop()
		if has then
			newline()
		end
		write(c.StructClose)
	end

	function encodeIdent(s: string)
		if not string.match(s, "^[A-Za-z_][0-9A-Za-z_]*$") then
			error("invalid identifier")
		end
		write(s)
	end

	encodeValue(value)
	return table.concat(buf)
end

return table.freeze(export)
