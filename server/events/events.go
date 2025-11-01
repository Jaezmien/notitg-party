package events

import (
	"encoding/json"

	. "git.jaezmien.com/Jaezmien/notitg-party/server/global"
)

type EventType string

const (
	EVENT_USER_SONG_STATE EventType = "room.user.song"
	EVENT_USER_STATE      EventType = "room.user.state"
	EVENT_USER_READY      EventType = "room.game.ready"
	EVENT_USER_SCORE      EventType = "room.game.score"
	EVENT_USER_FINISH     EventType = "room.game.finish"

	EVENT_ROOM_SONG  EventType = "room.song"
	EVENT_ROOM_START EventType = "room.start"
)

type RawEvent struct {
	Type EventType       `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func newEvent(t string, data any) []byte {
	return JSONMustByte(Event{
		Type: t,
		Data: data,
	})
}

func NewUserInfoEvent(username string, id string) []byte {
	return newEvent(
		"self.user",
		struct {
			ID string `json:"id"`
		}{
			ID: id,
		},
	)
}
func NewUserJoinEvent(username string, id string, state int) []byte {
	return newEvent(
		"room.user.join",
		struct {
			Username string `json:"username"`
			ID       string `json:"id"`
			State    int    `json:"state"`
		}{
			Username: username,
			ID:       id,
			State:    state,
		},
	)
}
func NewUserLeaveEvent(id string) []byte {
	return newEvent(
		"room.user.leave",
		struct {
			ID string `json:"id"`
		}{
			ID: id,
		},
	)
}
func NewUserStateEvent(id string, state int) []byte {
	return newEvent(
		"room.user.state",
		struct {
			ID    string `json:"id"`
			State int    `json:"state"`
		}{
			ID:    id,
			State: state,
		},
	)
}

func NewRoomIDEvent(id string) []byte {
	return newEvent(
		"room.info.id",
		struct {
			ID string `json:"id"`
		}{
			ID: id,
		},
	)
}
func NewRoomTitleEvent(title string) []byte {
	return newEvent(
		"room.info.title",
		struct {
			Title string `json:"title"`
		}{
			Title: title,
		},
	)
}
func NewRoomHostEvent(id string) []byte {
	return newEvent(
		"room.info.host",
		struct {
			ID string `json:"id"`
		}{
			ID: id,
		},
	)
}

type SongEventData struct {
	Hash       string `json:"hash"`
	Difficulty int    `json:"difficulty"`
}

func NewRoomSongEvent(hash string, difficulty int) []byte {
	return newEvent(
		"room.info.song",
		SongEventData{
			Hash:       hash,
			Difficulty: difficulty,
		},
	)
}

type UserSongEventData struct {
	HasSong bool `json:"has_song"`
}

func NewRoomStateEvent(state int) []byte {
	return newEvent(
		"room.state",
		struct {
			State int `json:"state"`
		}{
			State: state,
		},
	)
}

type UserStateEventData struct {
	State int `json:"state"`
}

func NewRoomStartEvent() []byte {
	return newEvent(
		"room.start",
		struct{}{},
	)
}

func NewGameplayStartEvent() []byte {
	return newEvent(
		"room.game.start",
		struct{}{},
	)
}

type PartialGameplayScoreEventData struct {
	Score int32 `json:"score"`
}
type GameplayScoreEventData struct {
	ID    string `json:"id"`
	PartialGameplayFinishEventData
}

func NewGameplayScoreEvent(id string, score int32) []byte {
	return newEvent(
		"room.game.score",
		GameplayScoreEventData{
			ID:    id,
			PartialGameplayFinishEventData: PartialGameplayFinishEventData{
				Score: score,
			},
		},
	)
}

type PartialGameplayFinishEventData struct {
	Score     int32 `json:"score"`
	Marvelous int32 `json:"marvelous"`
	Perfect   int32 `json:"perfect"`
	Great     int32 `json:"great"`
	Good      int32 `json:"good"`
	Boo       int32 `json:"boo"`
	Miss      int32 `json:"miss"`
}
type GameplayFinishEventData struct {
	ID        string `json:"id"`
	PartialGameplayFinishEventData
}
type JudgmentScore struct {
	Marvelous int32
	Perfect   int32
	Great     int32
	Good      int32
	Boo       int32
	Miss      int32
}

func NewGameplayFinishEvent(id string, score int32, judgment JudgmentScore) []byte {
	return newEvent(
		"room.game.finish",
		GameplayFinishEventData{
			ID:        id,
			PartialGameplayFinishEventData: PartialGameplayFinishEventData{
				Score:     score,
				Marvelous: judgment.Marvelous,
				Perfect:   judgment.Perfect,
				Great:     judgment.Great,
				Good:      judgment.Good,
				Boo:       judgment.Boo,
				Miss:      judgment.Miss,
			},
		},
	)
}

func NewEvaluationRevealEvent() []byte {
	return newEvent(
		"room.eval.show",
		struct{}{},
	)
}
