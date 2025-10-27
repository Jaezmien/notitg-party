package main

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"git.jaezmien.com/Jaezmien/notitg-party/server/events"
	"github.com/gorilla/websocket"
	// . "git.jaezmien.com/Jaezmien/notitg-party/server/global"
)

type ClientState int

const (
	CLIENT_IDLE ClientState = iota
	CLIENT_MISSING_SONG
	CLIENT_LOBBY_READY
	CLIENT_GAME_LOADING
	CLIENT_GAME_READY
	CLIENT_PLAYING
	CLIENT_RESULTS
)

type Client struct {
	Connection *websocket.Conn
	Room       *Room

	UUID     string
	Username string
	Host     bool

	InMatch bool

	Send   chan []byte
	Closed bool

	State ClientState
}

func (c *Client) Close() {
	if c.Closed {
		return
	}

	logger.Info("closing client", slog.String("id", c.UUID))

	c.Closed = true
	c.Connection.Close()
	close(c.Send)
}

func (c *Client) SetNewState(state ClientState) {
	c.State = state
	c.Room.BroadcastAll(events.NewUserStateEvent(c.UUID, int(state)))
}

func (c *Client) Write() {
	for message := range c.Send {
		w, err := c.Connection.NextWriter(websocket.TextMessage)
		if err != nil {
			return
		}
		w.Write(message)

		if err := w.Close(); err != nil {
			return
		}
	}
}
func (c *Client) Read() {
	for {
		t, message, err := c.Connection.ReadMessage()

		if c.Closed {
			return
		}
		if err != nil {
			logger.Error("error while reading client message", slog.Any("err", err))
			break
		}

		if t != websocket.TextMessage {
			logger.Error("unknown client message (non-text message)")
			break
		}

		logger.Info(fmt.Sprintf("received: %s", message))

		var baseEvent events.Event
		if err := json.Unmarshal(message, &baseEvent); err != nil {
			logger.Error("error while parsing client message", slog.Any("err", err))
			break
		}

		switch baseEvent.Type {
		case "room.song":
			var event struct {
				Type string               `json:"type"`
				Data events.SongEventData `json:"data"`
			}

			err := json.Unmarshal(message, &event)
			if err != nil {
				logger.Info("invalid client data", slog.Any("err", err))
				break
			}

			if !c.Room.IsInLobby() {
				logger.Info("room not in lobby, ignoring song change")
				break
			}
			if !c.Host {
				logger.Info("client is not host, ignoring")
				break
			}

			logger.Info("changing song!")
			c.Room.SetSong(event.Data.Hash, event.Data.Difficulty)
		case "room.user.song":
			var event struct {
				Type string                   `json:"type"`
				Data events.UserSongEventData `json:"data"`
			}

			err := json.Unmarshal(message, &event)
			if err != nil {
				logger.Info("invalid client data", slog.Any("err", err))
				break
			}

			if !c.Room.IsInLobby() {
				break
			}

			if event.Data.HasSong {
				c.SetNewState(CLIENT_IDLE)
			} else {
				c.SetNewState(CLIENT_MISSING_SONG)
			}

			c.Room.BroadcastAll(events.NewUserStateEvent(c.UUID, int(c.State)))
		case "room.user.state":
			var event struct {
				Type string                    `json:"type"`
				Data events.UserStateEventData `json:"data"`
			}

			err := json.Unmarshal(message, &event)
			if err != nil {
				logger.Info("invalid client data", slog.Any("err", err))
				break
			}

			if c.State == CLIENT_MISSING_SONG {
				break
			}

			if event.Data.State == 0 {
				c.SetNewState(CLIENT_IDLE)
			} else {
				c.SetNewState(CLIENT_LOBBY_READY)
			}
		case "room.start":
			if !c.Host {
				break
			}

			if !c.Room.IsReadyToStart() {
				break
			}

			for cl := range c.Room.Clients {
				if cl.State == CLIENT_MISSING_SONG {
					continue
				}
				cl.SetNewState(CLIENT_GAME_LOADING)

				cl.InMatch = true
				cl.Send <- events.NewRoomStartEvent()
			}

			c.Room.SetNewState(ROOM_PLAYING)
			logger.Info("room is setting up for gameplay", slog.String("id", c.Room.UUID))
		case "room.game.ready":
			if c.State != CLIENT_GAME_LOADING {
				break
			}

			c.SetNewState(CLIENT_GAME_READY)

			if c.Room.IsReadyToPlay() {
				for cl := range c.Room.Clients {
					if !cl.InMatch {
						continue
					}
					cl.SetNewState(CLIENT_PLAYING)
					cl.Send <- events.NewGameplayStartEvent()
				}
			}

			logger.Info("room has started playing", slog.String("id", c.Room.UUID))
		case "room.game.score":
			if !c.InMatch {
				break
			}
			if c.State != CLIENT_PLAYING {
				break
			}

			var event struct {
				Type string                               `json:"type"`
				Data events.PartialGameplayScoreEventData `json:"data"`
			}

			err := json.Unmarshal(message, &event)
			if err != nil {
				logger.Info("invalid client data", slog.Any("err", err))
				break
			}

			for cl := range c.Room.Clients {
				if !cl.InMatch {
					continue
				}
				if cl.UUID == c.UUID {
					continue
				}
				cl.Send <- events.NewGameplayScoreEvent(c.UUID, event.Data.Score)
			}
		case "room.game.finish":
			if !c.InMatch {
				break
			}
			if c.State != CLIENT_PLAYING {
				break
			}

			var event struct {
				Type string                                `json:"type"`
				Data events.PartialGameplayFinishEventData `json:"data"`
			}

			err := json.Unmarshal(message, &event)
			if err != nil {
				logger.Info("invalid client data", slog.Any("err", err))
				break
			}

			for cl := range c.Room.Clients {
				if !cl.InMatch {
					continue
				}
				if cl.UUID == c.UUID {
					continue
				}

				cl.Send <- events.NewGameplayFinishEvent(
					c.UUID,
					event.Data.Score, events.JudgmentScore{
						Marvelous: event.Data.Marvelous,
						Perfect:   event.Data.Perfect,
						Great:     event.Data.Great,
						Good:      event.Data.Good,
						Boo:       event.Data.Boo,
						Miss:      event.Data.Miss,
					},
				)
			}

			c.SetNewState(CLIENT_RESULTS)
			c.Room.BroadcastAll(events.NewRoomStateEvent(int(CLIENT_RESULTS)))

			logger.Info("player has finished song", slog.String("id", c.Room.UUID))

			if c.Room.IsAllFinished() {
				logger.Info("room has finished song", slog.String("id", c.Room.UUID))

				for cl := range c.Room.Clients {
					if !cl.InMatch {
						continue
					}

					cl.InMatch = false
					cl.SetNewState(CLIENT_IDLE)

					cl.Send <- events.NewEvaluationRevealEvent()
				}

				c.Room.SetNewState(ROOM_IDLE)
			} else {
				logger.Info("waiting for other players to finish...", slog.String("id", c.Room.UUID))
			}
		}
	}

	c.Room.Leave <- c
}
