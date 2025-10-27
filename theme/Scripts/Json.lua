--
-- json.lua
--
-- Copyright (c) 2019 rxi
--
-- Permission is hereby granted, free of charge, to any person obtaining a copy of
-- this software and associated documentation files (the "Software"), to deal in
-- the Software without restriction, including without limitation the rights to
-- use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
-- of the Software, and to permit persons to whom the Software is furnished to do
-- so, subject to the following conditions:
--
-- The above copyright notice and this permission notice shall be included in all
-- copies or substantial portions of the Software.
--
-- THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
-- IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
-- FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
-- AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
-- LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
-- OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
-- SOFTWARE.

-- edited by jaezmien to support OpenITG+ :D
-- + added sleeparrow's pretty print pull req, used as a base. (https://github.com/rxi/json.lua/pull/18)
json = {_version = "0.1.2"}

-------------------------------------------------------------------------------
-- Encode
-------------------------------------------------------------------------------

local encode

local escape_char_map = {
    ["\\"] = "\\\\",
    ["\""] = "\\\"",
    ["\b"] = "\\b",
    ["\f"] = "\\f",
    ["\n"] = "\\n",
    ["\r"] = "\\r",
    ["\t"] = "\\t"
}

local escape_char_map_inv = {["\\/"] = "/"}
for k, v in pairs(escape_char_map) do escape_char_map_inv[v] = k end

local function escape_char(c)
    return escape_char_map[c] or string.format("\\u%04x", string.byte(c))
end

local function encode_nil(val) return "null" end

local function make_indent(cur_indent,indent_level)
    return string.rep( " " , cur_indent * indent_level )
end
local function encode_table(val, state)
    local res = {}
    local stack = state.stack
    local pretty = state.indent_level > 0

    local indent = make_indent( state.current_indent_level , state.indent_level )
    local comma = "," .. (pretty and "\n" or "")
    local colon = ":" .. (pretty and " " or "")
    local open_brace = (pretty and indent or "") .. "{" .. (pretty and "\n" or "")
    local close_brace = (pretty and "\n"..indent or "") .. "}"
    local open_bracket = (pretty and indent or "") .. "[" .. (pretty and "\n" or "")
    local close_bracket = (pretty and "\n"..indent or "") .. "]"

    -- Circular reference?
    if state[val] then error("[json.lua] circular reference") end

    stack[val] = true

    if rawget(val, 1) ~= nil or next(val) == nil then
        -- Treat as array -- check keys are valid and it is not sparse
        local n = 0
        for k in pairs(val) do
            if type(k) ~= "number" then
                error("[json.lua] invalid table: mixed or invalid key types")
            end
            n = n + 1
        end
        if n ~= #val then error("[json.lua] invalid table: sparse array") end
        -- Encode
        for i, v in ipairs(val) do
            state.current_indent_level = state.current_indent_level + 1
            table.insert(res, encode(v, state))
            state.current_indent_level = state.current_indent_level - 1
        end
        stack[val] = nil
        return open_bracket .. table.concat(res, "," .. (pretty and "\n" or "")) .. close_bracket

    else
        -- Treat as an object
        for k, v in pairs(val) do
            if type(k) ~= "string" then
                error("[json.lua] invalid table: mixed or invalid key types")
            end
            state.current_indent_level = state.current_indent_level + 1
            table.insert(res, encode(k, state) .. ":" .. encode(v, state, true))
            state.current_indent_level = state.current_indent_level - 1
        end
        stack[val] = nil
        return open_brace .. table.concat(res, "," .. (pretty and "\n" or "")) .. close_brace
    end
end

local function encode_string(val, state)
    local indent = make_indent( state.current_indent_level , state.indent_level )
    return indent .. '"' .. string.gsub(val,'[%z\1-\31\\"]', escape_char) .. '"'
end


local math_huge = math.pow(2,1024)-1 -- thanks lua <5.0
local function encode_number(val, state)
    -- Check for NaN, -inf and inf
    if val ~= val or val <= -math_huge or val >= math_huge then
        error("[json.lua] unexpected number value '" .. tostring(val) .. "'")
    end
    local indent = make_indent( state.current_indent_level , state.indent_level )
    return indent .. string.format("%.14g", val)
end

local type_func_map = {
    ["nil"] = encode_nil,
    ["table"] = encode_table,
    ["string"] = encode_string,
    ["number"] = encode_number,
    ["boolean"] = tostring
}

encode = function(val, state, force_no_indentation)
    force_no_indentation = force_no_indentation or false
    local t = type(val)
    local f = type_func_map[t]
    if f then
        local old_indent = state.indent_level
        if force_no_indentation then state.indent_level = 0 end
        local val = f(val, state)
        state.indent_level = old_indent
        return val
    end
    error("[json.lua] unexpected type '" .. t .. "'")
end

function json.encode(val, indent_level, no_indent)
    local state = {
        stack = {},
        indent_level = indent_level or 0,
        current_indent_level = 0,
    }
    return encode(val, state, no_indent)
end

-------------------------------------------------------------------------------
-- Decode
-------------------------------------------------------------------------------

local parse

local function create_set(...)
    local res = {}
    for i = 1, #arg do res[arg[i]] = true end
    return res
end

local space_chars = create_set(" ", "\t", "\r", "\n")
local delim_chars = create_set(" ", "\t", "\r", "\n", "]", "}", ",")
local escape_chars = create_set("\\", "/", '"', "b", "f", "n", "r", "t", "u")
local literals = create_set("true", "false", "null")

local literal_map = {["true"] = true, ["false"] = false, ["null"] = nil}

local function next_char(str, idx, set, negate)
    for i = idx, #str do
        if set[ string.sub(str,i,i) ] ~= negate then
            return i
        end
    end
    return #str + 1
end

local function decode_error(str, idx, msg)
    local line_count = 1
    local col_count = 1
    for i = 1, idx - 1 do
        col_count = col_count + 1
        if string.sub(str, i, i) == "\n" then
            line_count = line_count + 1
            col_count = 1
        end
    end
    error(string.format("%s at line %d col %d", msg, line_count, col_count))
end

local function codepoint_to_utf8(n)
    -- http://scripts.sil.org/cms/scripts/page.php?site_id=nrsi&id=iws-appendixa
    local f = math.floor
    if n <= 0x7f then
        return string.char(n)
    elseif n <= 0x7ff then
        return string.char(f(n / 64) + 192, n % 64 + 128)
    elseif n <= 0xffff then
        return string.char(f(n / 4096) + 224, f(n % 4096 / 64) + 128,
                           n % 64 + 128)
    elseif n <= 0x10ffff then
        return string.char(f(n / 262144) + 240, f(n % 262144 / 4096) + 128,
                           f(n % 4096 / 64) + 128, n % 64 + 128)
    end
    error(string.format("invalid unicode codepoint '%x'", n))
end

--

    local function parse_unicode_escape(s)
        local n1 = tonumber(string.sub(s,3, 6), 16)
        local n2 = tonumber(string.sub(s,9, 12), 16)
        -- Surrogate pair?
        if n2 then
            return
                codepoint_to_utf8((n1 - 0xd800) * 0x400 + (n2 - 0xdc00) + 0x10000)
        else
            return codepoint_to_utf8(n1)
        end
    end

    local function parse_string(str, i)
        local has_unicode_escape = false
        local has_surrogate_escape = false
        local has_escape = false
        local last
        for j = i + 1, #str do
            local x = string.byte(str,j)

            if x < 32 then
                decode_error(str, j, "control character in string")
            end

            if last == 92 then -- "\\" (escape char)
                if x == 117 then -- "u" (unicode escape sequence)
                    local hex = string.sub(str,j + 1, j + 5)
                    if not string.find(hex,"%x%x%x%x") then
                        decode_error(str, j, "invalid unicode escape in string")
                    end
                    if string.find(hex,"^[dD][89aAbB]") then
                        has_surrogate_escape = true
                    else
                        has_unicode_escape = true
                    end
                else
                    local c = string.char(x)
                    if not escape_chars[c] then
                        decode_error(str, j,
                                    "invalid escape char '" .. c .. "' in string")
                    end
                    has_escape = true
                end
                last = nil

            elseif x == 34 then -- '"' (end of string)
                local s = string.sub(str,i + 1, j - 1)
                if has_surrogate_escape then
                    s = string.gsub(s,"\\u[dD][89aAbB]..\\u....", parse_unicode_escape)
                end
                if has_unicode_escape then
                    s = string.gsub(s,"\\u....", parse_unicode_escape)
                end
                if has_escape then s = string.gsub(s,"\\.", escape_char_map_inv) end
                return s, j + 1

            else
                last = x
            end
        end
        decode_error(str, i, "expected closing quote for string")
    end

    local function parse_number(str, i)
        local x = next_char(str, i, delim_chars)
        local s = string.sub(str,i, x - 1)
        local n = tonumber(s)
        if not n then decode_error(str, i, "invalid number '" .. s .. "'") end
        return n, x
    end

    local function parse_literal(str, i)
        local x = next_char(str, i, delim_chars)
        local word = string.sub(str,i, x - 1)
        if not literals[word] then
            decode_error(str, i, "invalid literal '" .. word .. "'")
        end
        return literal_map[word], x
    end

    local function parse_array(str, i)
        local res = {}
        local n = 1
        i = i + 1
        while 1 do
            local x
            i = next_char(str, i, space_chars, true)
            -- Empty / end of array?
            if string.sub(str,i, i) == "]" then
                i = i + 1
                break
            end
            -- Read token
            x, i = parse(str, i)
            res[n] = x
            n = n + 1
            -- Next token
            i = next_char(str, i, space_chars, true)
            local chr = string.sub(str,i, i)
            i = i + 1
            if chr == "]" then break end
            if chr ~= "," then decode_error(str, i, "expected ']' or ','") end
        end
        return res, i
    end

    local function parse_object(str, i)
        local res = {}
        i = i + 1
        while 1 do
            local key, val
            i = next_char(str, i, space_chars, true)
            -- Empty / end of object?
            if string.sub(str,i, i) == "}" then
                i = i + 1
                break
            end
            -- Read key
            if string.sub(str,i, i) ~= '"' then
                decode_error(str, i, "expected string for key")
            end
            key, i = parse(str, i)
            -- Read ':' delimiter
            i = next_char(str, i, space_chars, true)
            if string.sub(str,i, i) ~= ":" then
                decode_error(str, i, "expected ':' after key")
            end
            i = next_char(str, i + 1, space_chars, true)
            -- Read value
            val, i = parse(str, i)
            -- Set
            res[key] = val
            -- Next token
            i = next_char(str, i, space_chars, true)
            local chr = string.sub(str,i, i)
            i = i + 1
            if chr == "}" then break end
            if chr ~= "," then decode_error(str, i, "expected '}' or ','") end
        end
        return res, i
    end

--

local char_func_map = {
    ['"'] = parse_string,
    ["0"] = parse_number,
    ["1"] = parse_number,
    ["2"] = parse_number,
    ["3"] = parse_number,
    ["4"] = parse_number,
    ["5"] = parse_number,
    ["6"] = parse_number,
    ["7"] = parse_number,
    ["8"] = parse_number,
    ["9"] = parse_number,
    ["-"] = parse_number,
    ["t"] = parse_literal,
    ["f"] = parse_literal,
    ["n"] = parse_literal,
    ["["] = parse_array,
    ["{"] = parse_object
}

parse = function(str, idx)
    local chr = string.sub(str,idx, idx)
    local f = char_func_map[chr]
    if f then return f(str, idx) end
    decode_error(str, idx, "unexpected character '" .. chr .. "'")
end

function json.decode(str)
    if type(str) ~= "string" then
        error("[json.lua] expected argument of type string, got " .. type(str))
    end
    local res, idx = parse(str, next_char(str, 1, space_chars, true))
    idx = next_char(str, idx, space_chars, true)
    if idx <= #str then decode_error(str, idx, "trailing garbage") end
    return res
end
