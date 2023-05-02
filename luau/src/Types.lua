local export = {}

local cache = table.freeze{__mode="v"}

--------------------------------------------------------------------------------
--------------------------------------------------------------------------------

local null = table.freeze{__type="rod.null", __tostring=function() return "null" end}
local _null = table.freeze(setmetatable({}, null))

--- Returns a table that will be encoded as a ROD null.
---
--- @param v -- The null value.
function export.null(): any
	return _null
end

--------------------------------------------------------------------------------
--------------------------------------------------------------------------------

local bool = table.freeze{__type="rod.bool", __tostring=function(self) return tostring(self.Value) end}
local _true = table.freeze(setmetatable({Value=true}, bool))
local _false = table.freeze(setmetatable({Value=false}, bool))

--- Returns a table that will be encoded as a ROD bool.
---
--- @param v -- The bool value.
function export.bool(v: boolean): any
	return if v then _true else _false
end

--------------------------------------------------------------------------------
--------------------------------------------------------------------------------

local function toint(i)
	--TODO: Benchmark against math.modf.
	if i < 0 then
		return math.ceil(i)
	else
		return math.floor(i)
	end
end

local int = table.freeze{__type="rod.int", __tostring=function(self) return tostring(self.Value) end}
local intCache = setmetatable({}, cache)

--- Returns a table that will be encoded as a ROD int.
---
--- @param v -- The int value.
function export.int(v: number): any
	v = toint(v)
	local result = intCache[v]
	if result then
		return result
	end
	return table.freeze(setmetatable({Value = v}, int))
end

--------------------------------------------------------------------------------
--------------------------------------------------------------------------------

local float = table.freeze{__type="rod.float", __tostring=function(self) return tostring(self.Value) end}
local floatCache = setmetatable({}, cache)

--- Returns a table that will be encoded as a ROD float.
---
--- @param v -- The float value.
function export.float(v: number): any
	local result = floatCache[v]
	if result then
		return result
	end
	return table.freeze(setmetatable({Value = v}, float))
end

--------------------------------------------------------------------------------
--------------------------------------------------------------------------------

local string = table.freeze{__type="rod.string", __tostring=function(self) return string.format("%q",self.Value) end}
local stringCache = setmetatable({}, cache)

--- Returns a table that will be encoded as a ROD string.
---
--- @param v -- The string value.
function export.string(v: string): any
	local result = stringCache[v]
	if result then
		return result
	end
	return table.freeze(setmetatable({Value = v}, string))
end

--------------------------------------------------------------------------------
--------------------------------------------------------------------------------

local blob = table.freeze{__type="rod.blob", __tostring=function(self) return string.format("%q",self.Value) end}
local blobCache = setmetatable({}, cache)

--- Returns a table that will be encoded as a ROD blob.
---
--- @param v -- The blob value.
function export.blob(v: string): any
	local result = blobCache[v]
	if result then
		return result
	end
	return table.freeze(setmetatable({Value = v}, blob))
end

--------------------------------------------------------------------------------
--------------------------------------------------------------------------------

local array = table.freeze{__type="rod.array"}

--- Returns a table that will be encoded as a ROD array.
---
--- @param v -- An optional value to use as the array.
function export.array(v: {any}?): {any}
	if v == nil then
		return setmetatable({}, array) :: any
	elseif type(v) ~= "table" then
		error("table expected", 2)
	else
		return setmetatable(v, array) :: any
	end
end

--------------------------------------------------------------------------------
--------------------------------------------------------------------------------

local map = table.freeze{__type="rod.map"}

--- Returns a table that will be encoded as a ROD map.
---
--- @param v -- An optional value to use as the map.
function export.map(v: {[any]: any}?): {[any]: any}
	if v == nil then
		return setmetatable({}, map) :: any
	elseif type(v) ~= "table" then
		error("table expected", 2)
	else
		return setmetatable(v, map) :: any
	end
end

--------------------------------------------------------------------------------
--------------------------------------------------------------------------------

local struct = table.freeze{__type="rod.struct"}

--- Returns a table that will be encoded as a ROD struct.
---
--- @param v -- An optional value to use as the struct.
function export.struct(v: {[string]: any}?): {[string]: any}
	if v == nil then
		return setmetatable({}, struct) :: any
	elseif type(v) ~= "table" then
		error("table expected", 2)
	else
		return setmetatable(v, struct) :: any
	end
end

--------------------------------------------------------------------------------
--------------------------------------------------------------------------------

--- Returns the ROD type of the given value. Returns nil if `v` is not a
--- compatible ROD type. The following possible values can be returned:
---
--- - "null"
---     - A table where metamethod `__type` equals "rod.null".
---     - A nil value.
--- - "bool"
---     - A table where metamethod `__type` equals "rod.bool" and field "Value" is a boolean.
---     - A boolean value.
--- - "int"
---     - A table where metamethod `__type` equals "rod.int" and field "Value" is a number.
--- - "float"
---     - A table where metamethod `__type` equals "rod.float" and field "Value" is a number.
---     - A number value.
--- - "string"
---     - A table where metamethod `__type` equals "rod.string" and field "Value" is a string.
---     - A string value.
--- - "blob"
---     - A table where metamethod `__type` is "rod.blob" and field "Value" is a string.
--- - "array"
---     - A table where metamethod `__type` is "rod.array".
---     - A table with a length greater than 0.
--- - "struct"
---     - A table where metamethod `__type` is "rod.struct".
---     - A table where all keys are strings.
--- - "map"
---     - A table where metamethod `__type` is "rod.map".
---     - A table that does not match any other types.
---
function export.typeof(v: any): (string?, any)
	if v == nil then
		return "null", nil
	elseif type(v) == "boolean" then
		return "bool", v
	elseif type(v) == "number" then
		return "float", v
	elseif type(v) == "string" then
		return "string", v
	elseif type(v) ~= "table" then
		return nil
	end
	local mt = getmetatable(v)
	if type(mt) == "table" then
		if mt.__type == "rod.int" and type(v.Value) == "number" then
			return "int", v.Value
		elseif mt.__type == "rod.blob" and type(v.Value) == "string" then
			return "blob", v.Value
		elseif mt.__type == "rod.array" then
			return "array", v
		elseif mt.__type == "rod.map" then
			return "map", v
		elseif mt.__type == "rod.struct" then
			return "struct", v
		end
	end
	if v[1] ~= nil then
		return "array", v
	end
	for k in v do
		if type(k) ~= "string" then
			return "map", v
		end
	end
	return "struct", v
end

return table.freeze(export)
