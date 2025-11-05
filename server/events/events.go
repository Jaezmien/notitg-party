package events

import (
	"encoding/json"
	"fmt"

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

type BaseID struct {
	ID string `json:"id"`
}
type User struct {
	BaseID
	Username string `json:"username"`
}
type Title struct {
	Title string `json:"title"`
}
type SetSong struct {
	Hash       string `json:"hash"`
	Difficulty int    `json:"difficulty"`
}
type BaseState struct {
	State int `json:"state"`
}
type UserSongState struct {
	HasSong bool `json:"has_song"`
}
type UserJoin struct {
	User
	BaseState
}
type UserState struct {
	BaseID
	BaseState
}

type GameplayScore struct {
	Score int32 `json:"score"`
}
type GameplayScoreWithUserID struct {
	BaseID
	Score int32 `json:"score"`
}

type GameplayFinish struct {
	GameplayScore
	Marvelous int32 `json:"marvelous"`
	Perfect   int32 `json:"perfect"`
	Great     int32 `json:"great"`
	Good      int32 `json:"good"`
	Boo       int32 `json:"boo"`
	Miss      int32 `json:"miss"`
}
type GameplayFinishWithUserID struct {
	BaseID
	GameplayFinish
}

type Empty struct{}

func newEvent(t string, data any) []byte {
	return JSONMustByte(Event{
		Type: t,
		Data: data,
	})
}

func NewUserInfoEvent(username string, id string) []byte {
	return newEvent(
		"self.user",
		BaseID{id},
	)
}
func NewUserJoinEvent(username string, id string, state int) []byte {
	return newEvent(
		"room.user.join",
		UserJoin{
			User{BaseID{id}, username},
			BaseState{state},
		},
	)
}
func NewUserLeaveEvent(id string) []byte {
	return newEvent(
		"room.user.leave",
		BaseID{id},
	)
}

func NewUserStateEvent(id string, state int) []byte {
	return newEvent(
		"room.user.state",
		UserState{BaseID{id}, BaseState{state}},
	)
}
func ParseUserStateEvent(raw json.RawMessage) (UserState, error) {
	var data UserState

	err := json.Unmarshal(raw, &data)
	if err != nil {
		return data, fmt.Errorf("invalid json data: %w", err)
	}

	return data, nil
}
func ParseUserSongStateEvent(raw json.RawMessage) (UserSongState, error) {
	var data UserSongState

	err := json.Unmarshal(raw, &data)
	if err != nil {
		return data, fmt.Errorf("invalid json data: %w", err)
	}

	return data, nil
}

func NewRoomIDEvent(id string) []byte {
	return newEvent(
		"room.info.id",
		BaseID{id},
	)
}
func NewRoomTitleEvent(title string) []byte {
	return newEvent(
		"room.info.title",
		Title{title},
	)
}
func NewRoomHostEvent(id string) []byte {
	return newEvent(
		"room.info.host",
		BaseID{id},
	)
}

func NewRoomSongEvent(hash string, difficulty int) []byte {
	return newEvent(
		"room.info.song",
		SetSong{hash, difficulty},
	)
}
func ParseRoomSongEvent(raw json.RawMessage) (SetSong, error) {
	var data SetSong

	err := json.Unmarshal(raw, &data)
	if err != nil {
		return data, fmt.Errorf("invalid json data: %w", err)
	}

	if data.Difficulty < 0 {
		return data, fmt.Errorf("invalid difficulty")
	}

	return data, nil
}

func NewRoomStateEvent(state int) []byte {
	return newEvent(
		"room.state",
		BaseState{state},
	)
}

func NewRoomStartEvent() []byte {
	return newEvent(
		"room.start",
		Empty{},
	)
}

func NewGameplayStartEvent() []byte {
	return newEvent(
		"room.game.start",
		Empty{},
	)
}

func NewGameplayScoreEvent(id string, score int32) []byte {
	return newEvent(
		"room.game.score",
		GameplayScoreWithUserID{BaseID{id}, score},
	)
}
func ParseGameplayScoreEvent(raw json.RawMessage) (GameplayScore, error) {
	var data GameplayScore

	err := json.Unmarshal(raw, &data)
	if err != nil {
		return data, fmt.Errorf("invalid json data: %w", err)
	}

	if data.Score < 0 {
		return data, fmt.Errorf("invalid score value")
	}

	return data, nil
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
		GameplayFinishWithUserID{
			BaseID{id},
			GameplayFinish{
				GameplayScore{score},
				judgment.Marvelous,
				judgment.Perfect,
				judgment.Great,
				judgment.Good,
				judgment.Boo,
				judgment.Miss,
			},
		},
	)
}
func ParseGameplayFinishEvent(raw json.RawMessage) (GameplayFinish, error) {
	var data GameplayFinish

	err := json.Unmarshal(raw, &data)
	if err != nil {
		return data, fmt.Errorf("invalid json data: %w", err)
	}

	if data.Score < 0 {
		return data, fmt.Errorf("invalid score value")
	}
	if data.Marvelous < 0 {
		return data, fmt.Errorf("invalid judgment value")
	}
	if data.Perfect < 0 {
		return data, fmt.Errorf("invalid judgment value")
	}
	if data.Great < 0 {
		return data, fmt.Errorf("invalid judgment value")
	}
	if data.Good < 0 {
		return data, fmt.Errorf("invalid judgment value")
	}
	if data.Boo < 0 {
		return data, fmt.Errorf("invalid judgment value")
	}
	if data.Miss < 0 {
		return data, fmt.Errorf("invalid judgment value")
	}

	return data, nil
}

func NewEvaluationRevealEvent() []byte {
	return newEvent(
		"room.eval.show",
		Empty{},
	)
}
