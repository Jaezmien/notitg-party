--[[

	Lemonade v2.0 Mapping

	 | Range | Description             | Values                                                                                          | Notes                                        |
	| ----- | ----------------------- | ----------------------------------------------------------------------------------------------- | -------------------------------------------- |
	| 0     | Incoming Application ID | int                                                                                             | Application -> NotITG                        |
	| 1     | Incoming Type           | `0` = Partial, `1` = End of Buffer                                                              |                                              |
	| 2     | Incoming Length         | int                                                                                             |                                              |
	| 3-29  | Incoming Data           | int[]                                                                                           |                                              |
	| 30    | Incoming State          | `0` = Idle, `1` = Reserved, currently being written, `2` = Data is available for NotITG to read |                                              |
	| 31    | Outgoing Listener ID    | int                                                                                             | NotITG -> Application                        |
	| 32    | Outgoing Type           | `0` = Partial, `1` = End of Buffer                                                              |                                              |
	| 33    | Outgoing Length         | int                                                                                             |                                              |
	| 34-60 | Outgoing Data           | int[]                                                                                           |                                              |
	| 61    | Outgoing State          | `0` = Idle, `1` = Data is available for application to read                                     |                                              |
	| 63    | Initialization State    | Must be 56                                                                                      | Has NotITG initialized and finished loading? |

]]--

Lemonade = {}
Lemonade.Timer = nil

LEMONADE_INDEXES = {
	INCOMING = {
		ID = 0,
		TYPE = 1,
		LENGTH = 2,
		DATA = {3, 29},
		STATE = 30
	},
	OUTGOING = {
		ID = 31,
		TYPE = 32,
		LENGTH = 33,
		DATA = {34, 60},
		STATE = 61
	},
	INIT_STATE = 63,
}
LEMONADE_INCOMING_STATE = {
	IDLE = 0,
	BUSY = 1,
	AVAILABLE = 2, -- Meaning that data is present for NotITG to read!
}
LEMONADE_OUTGOING_STATE = {
	IDLE = 0,
	AVAILABLE = 1, -- Meaning that data is present for the application to read!
}
LEMONADE_BUFFER_TYPE = {
	PARTIAL = 0,
	END = 1,
}
LEMONADE_MAXIMUM_BUFFER_LENGTH = LEMONADE_INDEXES.INCOMING.DATA[2] - LEMONADE_INDEXES.INCOMING.DATA[1]
LEMONADE_INIT_VALUE = 56

Lemonade.Initialized = false
function Lemonade:Initialize()
	for i=0,63 do GAMESTATE:SetExternal(i,0) end
	GAMESTATE:SetExternal(LEMONADE_INDEXES.INIT_STATE, LEMONADE_INIT_VALUE)
	print('[Lemonade] Initialized!')
	self.Initialized = true
end

Lemonade.Enabled = true
---@param disable boolean
function Lemonade:Disable(disable) self.Enabled = not disable end

--- Encodes a string into a byte array
---@param str string
---@return number[]
function Lemonade:Encode(str)
	if type(str) ~= "string" then error(string.format("Expected type string, got %s", type(str))) return {} end
	local b = {}
	for i=1, string.len(str) do
		table.insert(b, string.byte(str, i))
	end
	return b
end

--- Decodes a byte array into a string
---@param buff number[]
---@return string
function Lemonade:Decode(buff)
	if type(buff) ~= "table" then error(string.format("Expected type table, got %s", type(buff))) return "" end
	local b = {}
	for _,v in ipairs(buff) do
		table.insert(b, string.char(v))
	end
	return table.concat(b, "")
end

--

---@type table<number, number[]>
local partialReadBuffers = {}

---@alias BufferType
---| 0 # Partial
---| 1 # End

---@class WriteBuffer
---@field appID number
---@field data number[]
---@field state BufferType

---@type WriteBuffer[]
local upcomingWriteBuffers = {}

---Keeps track of the last write buffer
---@type WriteBuffer | nil
local lastSeenWrite = nil

-- TODO: Validations

---Sends data
---@param appID number
---@param buffer number[]
function Lemonade:Send(appID, buffer)
	if type(appID) ~= 'number' then return error(string.format("Expected app id to be number, got %s", type(appID))) end

	if table.getn(buffer) <= LEMONADE_MAXIMUM_BUFFER_LENGTH then
		table.insert(upcomingWriteBuffers, {
			appID = appID,
			data = buffer,
			state = LEMONADE_BUFFER_TYPE.END
		})

		return
	end

	local idx = 1
	while idx <= table.getn(buffer) do
		local partialData = {}

		local i = idx
		while i <= math.min(table.getn(buffer),idx+(LEMONADE_MAXIMUM_BUFFER_LENGTH-1)) do
			table.insert(partialData, buffer[i])
			i = i + 1
		end

		local isEnd = idx + LEMONADE_MAXIMUM_BUFFER_LENGTH > table.getn(buffer)

		table.insert( upcomingWriteBuffers, {
			appID = appID,
			data = partialData,
			state = isEnd and LEMONADE_BUFFER_TYPE.END or LEMONADE_BUFFER_TYPE.PARTIAL
		} )

		idx = idx + LEMONADE_MAXIMUM_BUFFER_LENGTH
	end
end

---@type table<number, table<string, fun(data: number[])>>
local listeners = {}

---@param appID number
---@return table<string, fun(data: number[])>
function Lemonade:GetListeners(appID)
	if listeners[appID] == nil then return {} end
	return listeners[appID]
end
---@param appID number
---@return boolean
function Lemonade:HasListeners(appID)
	local n = 0
	for _,_ in ipairs(self:GetListeners(appID)) do
		n = n + 1
	end
	return n
end
---@param appID number
---@param callbackID string
---@param callback fun(data: number[])
function Lemonade:AddListener(appID, callbackID, callback)
	listeners[appID] = listeners[appID] or {}
	listeners[appID][callbackID] = callback
end
---@param appID number
---@param callbackID string
function Lemonade:RemoveListener(appID, callbackID)
	if listeners[appID] == nil then return end
	listeners[appID][callbackID] = nil
end

---Checks if the Outgoing Channel is still being used by the last write
---@return boolean
function Lemonade:IsOutgoingBlocked()
	if not lastSeenWrite then return false end

	local appID = GAMESTATE:GetExternal(LEMONADE_INDEXES.OUTGOING.ID)
	local state = GAMESTATE:GetExternal(LEMONADE_INDEXES.OUTGOING.STATE)

	if appID ~= lastSeenWrite.appID then return false end
	if state ~= lastSeenWrite.state then return false end

	local length = GAMESTATE:GetExternal(LEMONADE_INDEXES.OUTGOING.LENGTH)
	for i=1,length do
		local idx = LEMONADE_INDEXES.OUTGOING.DATA[1] + (i-1)
		local externalData = GAMESTATE:GetExternal(idx)
		if lastSeenWrite.data[i] ~= externalData then return false end
	end

	return true
end

function Lemonade:ClearOutgoingData()
	lastSeenWrite = nil

	for i=LEMONADE_INDEXES.OUTGOING.DATA[1],LEMONADE_INDEXES.OUTGOING.DATA[2] do GAMESTATE:SetExternal(i,0) end
	GAMESTATE:SetExternal(LEMONADE_INDEXES.OUTGOING.ID,0)
	GAMESTATE:SetExternal(LEMONADE_INDEXES.OUTGOING.TYPE,0)
	GAMESTATE:SetExternal(LEMONADE_INDEXES.OUTGOING.LENGTH,0)
	GAMESTATE:SetExternal(LEMONADE_INDEXES.OUTGOING.STATE,LEMONADE_OUTGOING_STATE.IDLE)
end

function Lemonade:Tick()
	if not self.Enabled or not self.Initialized then return end
	if not (FUCK_EXE and tonumber(GAMESTATE:GetVersionDate()) > 20180617) then
		self.Enabled = false
		return
	end

	if GAMESTATE:GetExternal(LEMONADE_INDEXES.INCOMING.STATE) == LEMONADE_INCOMING_STATE.AVAILABLE then
		local data = {}

		for i=1,GAMESTATE:GetExternal(LEMONADE_INDEXES.INCOMING.LENGTH) do
			local idx = LEMONADE_INDEXES.INCOMING.DATA[1] + (i-1)
			table.insert(data, GAMESTATE:GetExternal(idx) )
			GAMESTATE:SetExternal(idx, 0)
		end
		GAMESTATE:SetExternal(LEMONADE_INDEXES.INCOMING.LENGTH,0)

		local appID = GAMESTATE:GetExternal(LEMONADE_INDEXES.INCOMING.ID)

		local errs = {}
		if GAMESTATE:GetExternal(LEMONADE_INDEXES.INCOMING.TYPE) == LEMONADE_BUFFER_TYPE.END then
			if partialReadBuffers[appID] then
				for _,v in ipairs(data) do table.insert(partialReadBuffers[appID], v) end
				data = partialReadBuffers[appID]
				partialReadBuffers[appID] = nil
			end

			if self:HasListeners(appID) then
				for _, callback in pairs( self:GetListeners(appID) ) do
					local ok, err = pcall(callback, data)
					if not ok then table.insert(errs, err) end
				end
			else
				print(string.format('[Lemonade] %d has no listeners!', appID))
			end
		else
			partialReadBuffers[appID] = partialReadBuffers[appID] or {}
			for _,v in ipairs(data) do table.insert(partialReadBuffers[appID], v) end
		end

		GAMESTATE:SetExternal(LEMONADE_INDEXES.INCOMING.TYPE, 0)
		GAMESTATE:SetExternal(LEMONADE_INDEXES.INCOMING.ID, 0)
		GAMESTATE:SetExternal(LEMONADE_INDEXES.INCOMING.STATE, LEMONADE_INCOMING_STATE.IDLE)

		for _,v in ipairs(errs) do
			error(v)
		end
	end

	if GAMESTATE:GetExternal(LEMONADE_INDEXES.OUTGOING.STATE) == LEMONADE_OUTGOING_STATE.IDLE and
		table.getn( upcomingWriteBuffers ) > 0 then

		---@type WriteBuffer
		local writeInfo = table.remove(upcomingWriteBuffers,1)

		for i,v in ipairs(writeInfo.data) do
			local idx = LEMONADE_INDEXES.OUTGOING.DATA[1] + (i-1)
			GAMESTATE:SetExternal(idx, v)
		end

		GAMESTATE:SetExternal(LEMONADE_INDEXES.OUTGOING.LENGTH, table.getn(writeInfo.data))
		GAMESTATE:SetExternal(LEMONADE_INDEXES.OUTGOING.TYPE , writeInfo.state)
		GAMESTATE:SetExternal(LEMONADE_INDEXES.OUTGOING.ID , writeInfo.appID)
		GAMESTATE:SetExternal(LEMONADE_INDEXES.OUTGOING.STATE , LEMONADE_OUTGOING_STATE.AVAILABLE)

		lastSeenWrite = writeInfo
	end
end

function Lemonade:DumpState()
	print('Incoming ID', GAMESTATE:GetExternal(LEMONADE_INDEXES.INCOMING.ID))
	print('Incoming Type', GAMESTATE:GetExternal(LEMONADE_INDEXES.INCOMING.TYPE))
	print('Incoming Length', GAMESTATE:GetExternal(LEMONADE_INDEXES.INCOMING.LENGTH))
	print('Incoming State', GAMESTATE:GetExternal(LEMONADE_INDEXES.INCOMING.STATE))

	print('Outgoing ID', GAMESTATE:GetExternal(LEMONADE_INDEXES.OUTGOING.ID))
	print('Outgoing Type', GAMESTATE:GetExternal(LEMONADE_INDEXES.OUTGOING.TYPE))
	print('Outgoing Length', GAMESTATE:GetExternal(LEMONADE_INDEXES.OUTGOING.LENGTH))
	print('Outgoing State', GAMESTATE:GetExternal(LEMONADE_INDEXES.OUTGOING.STATE))
end

function Lemonade:CountAwaitingWrite()
	print(table.getn( upcomingWriteBuffers ))
end
function Lemonade:CleanAwaitingWrite()
	upcomingWriteBuffers = {}
end
