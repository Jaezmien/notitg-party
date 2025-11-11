package events

import (
	. "git.jaezmien.com/Jaezmien/notitg-party/client/global"
)

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

type SongEventData struct {
	Hash       string `json:"hash"`
	Difficulty string `json:"difficulty"`
}

func NewSetSongEvent(hash string, difficulty string) []byte {
	return newEvent(
		"room.song",
		SongEventData{
			Hash:       hash,
			Difficulty: difficulty,
		},
	)
}

type UserSongEventData struct {
	HasSong bool `json:"has_song"`
}

func NewUserSongEvent(hasSong bool) []byte {
	return newEvent(
		"room.user.song",
		UserSongEventData{
			HasSong: hasSong,
		},
	)
}

type UserStateEventData struct {
	State int `json:"state"`
}

func NewUserStateEvent(state int) []byte {
	return newEvent(
		"room.user.state",
		UserStateEventData{
			State: state,
		},
	)
}

func NewHostStartEvent() []byte {
	return newEvent(
		"room.start",
		struct{}{},
	)
}

func NewGameplayReadyEvent() []byte {
	return newEvent(
		"room.game.ready",
		struct{}{},
	)
}

type GameplayScoreEventData struct {
	Score int32 `json:"score"`
}

func NewGameplayScoreEvent(score int32) []byte {
	return newEvent(
		"room.game.score",
		GameplayScoreEventData{
			Score: score,
		},
	)
}

type EvaluationScoreEventData struct {
	Score     int32 `json:"score"`
	Marvelous int32 `json:"marvelous"`
	Perfect   int32 `json:"perfect"`
	Great     int32 `json:"great"`
	Good      int32 `json:"good"`
	Boo       int32 `json:"boo"`
	Miss      int32 `json:"miss"`
}
type JudgmentScore struct {
	Marvelous int32
	Perfect   int32
	Great     int32
	Good      int32
	Boo       int32
	Miss      int32
}

func NewGameplayFinishEvent(score int32, judgment JudgmentScore) []byte {
	return newEvent(
		"room.game.finish",
		EvaluationScoreEventData{
			Score:     score,
			Marvelous: judgment.Marvelous,
			Perfect:   judgment.Perfect,
			Great:     judgment.Great,
			Good:      judgment.Good,
			Boo:       judgment.Boo,
			Miss:      judgment.Miss,
		},
	)
}
