PARTY_CMD = {}

-- HACK: Ensure functions can't run multiple times in quick succession
-- For some reason, calling code in Choice/NextScreen always runs multiple times
local function hush(timeout)
	-- peak desperation

	local seen = -1
	return function()
		local now = os.time()

		if now < seen + timeout then
			return true
		end

		seen = now
		return false
	end
end

PARTY_CMD.lobbyRooms = {}
PARTY_CMD.score = 0 -- just for tracking purposes

PARTY_CMD.ROOM_IDLE = 0
PARTY_CMD.ROOM_PLAYING = 1

PARTY_CMD.CLIENT_IDLE = 0
PARTY_CMD.CLIENT_MISSING_SONG = 1
PARTY_CMD.CLIENT_LOBBY_READY = 2
PARTY_CMD.CLIENT_GAME_LOADING = 3
PARTY_CMD.CLIENT_GAME_READY = 4
PARTY_CMD.CLIENT_PLAYING = 5
PARTY_CMD.CLIENT_RESULTS = 6

PARTY_CMD.drawLeaderboardBottom = false

PARTY_CMD.difficulties = {
	[DIFFICULTY_BEGINNER] = 'beginner',
	[DIFFICULTY_EASY] = 'easy',
	[DIFFICULTY_MEDIUM] = 'medium',
	[DIFFICULTY_HARD] = 'hard',
	[DIFFICULTY_CHALLENGE] = 'challenge',
	[DIFFICULTY_EDIT] = 'edit',
}

PARTY_CMD.room = {}
function PARTY_CMD:ResetRoomData()
	PARTY_CMD.room.userid = ''

	PARTY_CMD.room.id = ''
	PARTY_CMD.room.title = ''
	PARTY_CMD.room.hostid = ''
	PARTY_CMD.room.state = PARTY_CMD.ROOM_IDLE

	PARTY_CMD.room.users = {}
	PARTY_CMD.room.playingUsers = {}

	PARTY_CMD.room.hasSong = true

	PARTY_CMD.room.difficulty = DIFFICULTY_BEGINNER
end

function PARTY_CMD:IsInRoom()
	return PARTY_CMD.room.id ~= ''
end

function PARTY_CMD:FindUserByID(id)
	if not PARTY_CMD:IsInRoom() then return nil end
	for _,u in ipairs(PARTY_CMD.room.users) do
		if u.id == id then return u end
	end
end
function PARTY_CMD:FindPlayingUserByID(id)
	if not PARTY_CMD:IsInRoom() then return nil end
	for _,u in ipairs(PARTY_CMD.room.playingUsers) do
		if u.id == id then return u end
	end
end
function PARTY_CMD:GetOwnUser()
	return PARTY_CMD:FindUserByID(PARTY_CMD.room.userid)
end
function PARTY_CMD:GetRoomHost()
	return PARTY_CMD:FindUserByID(PARTY_CMD.room.hostid)
end
function PARTY_CMD:IsUserHost()
	return PARTY_CMD:IsInRoom() and PARTY_CMD.room.userid == PARTY_CMD.room.hostid
end

function PARTY_CMD:NewScreen(s)
	PARTY_CMD.lobbyRooms = {}

	PARTY_ACTOR:GetChild('PartyPlayers'):hidden( s == 'ScreenPartyGameplay' and 0 or 1 )
	PARTY_ACTOR:GetChild('LoadingText'):hidden( s == 'ScreenPartyGameplay' and 0 or 1 )
end

function PARTY_CMD:CreateRoom()
	Lemonade:Send(2, {2, 2})

	PARTY_CMD:ResetRoomData()
end
function PARTY_CMD:JoinRoom(id)
	local data = Lemonade:Encode(id)
	table.insert(data, 1, 3) -- {3, data...}
	table.insert(data, 1, 2) -- {2, 3, data...}
	Lemonade:Send(2, data)

	PARTY_CMD:ResetRoomData()
end

function PARTY_CMD:RoomAllReady()
	if table.getn(PARTY_CMD.room.users) == 0 then return false end

	for _,u in ipairs(PARTY_CMD.room.users) do
		if u.state ~= PARTY_CMD.CLIENT_LOBBY_READY and
			u.state ~= PARTY_CMD.CLIENT_MISSING_SONG then
				return false
		end
	end

	-- handle just in case every player in the lobby is missing the song
	local allMissing = true
	for _,u in ipairs(PARTY_CMD.room.users) do
		if u.state ~= PARTY_CMD.CLIENT_MISSING_SONG then
			allMissing = false
			break
		end
	end
	if allMissing then return false end

	return true
end

function PARTY_CMD:ToggleSelfState(id)
	local u = PARTY_CMD:GetOwnUser()
	if u == nil then return end

	if u.state == PARTY_CMD.CLIENT_IDLE then
		Lemonade:Send(2, {3, 4, 1}) -- Set our state to ready
	else
		Lemonade:Send(2, {3, 4, 0}) -- Set our state to idle
	end
end

function PARTY_CMD:StartRoom()
	if PARTY_CMD:GetOwnUser() == nil then return end
	if not PARTY_CMD:IsUserHost() then return end

	Lemonade:Send(2, {3, 5}) -- Let's get started!
end
function PARTY_CMD:IsRoomPlaying()
	return PARTY_CMD.room.state == PARTY_CMD.ROOM_PLAYING
end
function PARTY_CMD:GameplayReady()
	if PARTY_CMD:GetOwnUser() == nil then return end

	Lemonade:Send(2, {4, 1})
end
function PARTY_CMD:GameplayFinish()
	if PARTY_CMD:GetOwnUser() == nil then return end

	local stats = STATSMAN:GetCurStageStats():GetPlayerStageStats(PLAYER_1)

	local score = stats:GetActualDancePoints()
	local marvelous = stats:GetTapNoteScores(TNS_MARVELOUS)
	local perfect = stats:GetTapNoteScores(TNS_PERFECT)
	local great = stats:GetTapNoteScores(TNS_GREAT)
	local good = stats:GetTapNoteScores(TNS_GOOD)
	local boo = stats:GetTapNoteScores(TNS_BOO)
	local miss = stats:GetTapNoteScores(TNS_MISS)

	Lemonade:Send(2, {4, 3, score, marvelous, perfect, great, good, boo, miss})
end

local newSongHush = hush(3)
function PARTY_CMD:SetNewSong()
	if newSongHush() then return end

	local song = GAMESTATE:GetCurrentSong()
	if not song then return end
	local _, _, folder, file = string.find(song:GetSongDir(), [[/([^/]+)/([^/]+)/$]])
	if not folder or not file then
		error('error in extracting song folder info')
		return
	end

	local jsonData = {
		key = folder ..'/'.. file,
		difficulty = GAMESTATE:PlayerDifficulty(PLAYER_1)
	}

	local data = Lemonade:Encode(json.encode(jsonData))
	table.insert(data, 1, 2) -- {2, data...}
	table.insert(data, 1, 3) -- {3, 2, data...}
	Lemonade:Send(2, data)
end

function PARTY_CMD:BroadcastScore(score)
	Lemonade:Send(2, {4, 2, score})
end

local function popBuffer(buf, n)
	local data = {}
	for i,v in ipairs(buf) do
		if i > n then table.insert(data, v) end
	end
	return data
end

Lemonade:AddListener(2, 'party', function(buffer)
	-- Miscellaneous
	if buffer[1] == 1 then
		if buffer[2] == 1 then
			-- Scenario: Client has successfully detected NotITG, now it's asking us to move the lobby
			-- We'll respond back by switching to the lobby screen.
			SCREENMAN:SetNewScreen('ScreenPartyLobby')
		elseif buffer[2] == 2 then
			-- Scenario: Client is possibly exiting, and needs to inform NotITG
			-- Let's send the acknowledgement, and hope that it receives it
			Lemonade:Send(2, {1, 2})
			SCREENMAN:SetNewScreen('ScreenTitleMenu')
		end
	end

	-- JSON!
	if buffer[1] == 99 then
		-- We have some JSON data!
		local rawData = popBuffer(buffer, 1)
		local jsonData = json.decode(Lemonade:Decode(rawData))

		if not jsonData.type then
			error('Unknown data!')
			return
		end

		if jsonData.type == 'self.user' then
			PARTY_CMD.room.userid = jsonData.data.id
		end
		if jsonData.type == 'room.info.id' then
			PARTY_CMD.room.id = jsonData.data.id
		end
		if jsonData.type == 'room.info.title' then
			PARTY_CMD.room.title = jsonData.data.title
		end
		if jsonData.type == 'room.info.host' then
			PARTY_CMD.room.hostid = jsonData.data.id
		end
		if jsonData.type == 'room.user.state' then
			local u = PARTY_CMD:FindUserByID(jsonData.data.id)
			if u then
				u.state = jsonData.data.state
			end
		end
		if jsonData.type == 'room.state' then
			PARTY_CMD.room.state = jsonData.data.state
		end
		if jsonData.type == 'room.info.song' then
			local hash = jsonData.data.hash
			PARTY_CMD.room.difficulty = jsonData.data.difficulty

			-- Throw back the hash to the client for verification
			PARTY_CMD.room.hasSong = true

			local data = Lemonade:Encode(hash)
			table.insert(data, 1, 3) -- {3, data...}
			table.insert(data, 1, 3) -- {3, 3, data...}
			Lemonade:Send(2, data)
		end

		if jsonData.type == 'room.user.join' then
			table.insert(PARTY_CMD.room.users, {
				username = jsonData.data.username,
				id = jsonData.data.id,
				state = jsonData.data.state,
			})
		end
		if jsonData.type == 'room.user.leave' then
			for i,v in ipairs(PARTY_CMD.room.users) do
				if v.id == jsonData.data.id then
					table.remove(PARTY_CMD.room.users, i)
					break
				end
			end

			local pu = PARTY_CMD:FindPlayingUserByID(jsonData.data.id)
			if pu then
				pu.left = true
			end
		end
		if jsonData.type == 'room.start' then
			if not PARTY_CMD.room.hasSong then return end

			PARTY_CMD.room.playingUsers = {}

			local idx = 1
			for i,v in ipairs(PARTY_CMD.room.users) do
				if v.state == PARTY_CMD.CLIENT_LOBBY_READY or
					v.state == PARTY_CMD.CLIENT_GAME_LOADING then
					table.insert(PARTY_CMD.room.playingUsers, {
						username = v.username,
						id = v.id,
						left = false,
						score = 0,
						index = idx,
						judgments = nil,
					})

					idx = idx + 1
				end
			end
			PARTY_CMD.score = 0

			InitializeMods() -- simply love thing

			SCREENMAN:SetNewScreen('ScreenPartyGameplay')
		end
		if jsonData.type == 'room.game.start' then
			SCREENMAN:GetTopScreen():PauseGame(false)
			MESSAGEMAN:Broadcast('PartyGameplayStart')
		end
		if jsonData.type == 'room.game.score' then
			local u = PARTY_CMD:FindPlayingUserByID(jsonData.data.id)
			if u then
				u.score = jsonData.data.score
			end
		end
		if jsonData.type == 'room.game.finish' then
			local u = PARTY_CMD:FindPlayingUserByID(jsonData.data.id)
			if u then
				u.score = jsonData.data.score
				u.judgments = {
					jsonData.data.marvelous,
					jsonData.data.perfect,
					jsonData.data.great,
					jsonData.data.good,
					jsonData.data.boo,
					jsonData.data.miss,
				}
			end
		end
		if jsonData.type == 'room.eval.show' then
			MESSAGEMAN:Broadcast('PartyEvaluationShow')
		end
	end

	-- Lobby!
	if buffer[1] == 2 then
		if buffer[2] == 1 then
			-- If we get this, then it means that the rest of the message is the lobby state in JSON
			local rawData = popBuffer(buffer, 2)

			PARTY_CMD.lobbyRooms = json.decode(Lemonade:Decode(rawData))
		end
		if buffer[2] == 2 then
			-- We're in a room! Let's move to the room screen!
			GAMESTATE:SetCurrentSong(nil)
			SCREENMAN:SetNewScreen('ScreenPartyRoom')
		end
	end

	-- Room!
	if buffer[1] == 3 then
		if buffer[2] == 1 then
			-- Scenario: We asked the client if we have the song, let's see what we got!

			if buffer[3] == 1 then
				-- We... don't.
				PARTY_CMD.room.hasSong = false
				GAMESTATE:SetCurrentSong(nil)
			elseif buffer[3] == 2 then
				-- We do!
				PARTY_CMD.room.hasSong = true

				-- Let's prepare the ApplyGameCommands!
				local rawData = popBuffer(buffer, 3)
				local key = Lemonade:Decode(rawData)

				GAMESTATE:ApplyGameCommand('playmode,regular')
				GAMESTATE:ApplyGameCommand('song,' .. key)
				GAMESTATE:ApplyGameCommand('style,versus')
				GAMESTATE:ApplyGameCommand('steps,' .. PARTY_CMD.difficulties[PARTY_CMD.room.difficulty])

				-- TODO: Allow players to change their mod options?
				GAMESTATE:ApplyGameCommand('mod,scalable')
			end
		end
	end

	-- Gameplay!
	if buffer[1] == 4 then
	end
end)
