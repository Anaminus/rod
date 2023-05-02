local Types = require(script.Types)
local Decoder = require(script.Decoder)

local export = {}

export.null = Types.null
export.bool = Types.bool
export.int = Types.int
export.float = Types.float
export.string = Types.string
export.blob = Types.blob
export.array = Types.array
export.map = Types.map
export.struct = Types.struct

export.typeof = Types.typeof

export.decode = Decoder.decode

return table.freeze(export)
