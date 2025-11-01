package main

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"git.jaezmien.com/Jaezmien/notitg-party/server/events"
	"github.com/gorilla/websocket"
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

		var event events.RawEvent
		if err := json.Unmarshal(message, &event); err != nil {
			logger.Error("error while parsing client message", slog.Any("err", err))
			break
		}

		if event.Type != events.EVENT_USER_SCORE {
			logger.Info(fmt.Sprintf("received event: %s", event.Type))
		}

		switch event.Type {
		case events.EVENT_ROOM_SONG:
			var data events.SongEventData
			err := json.Unmarshal(event.Data, &data)
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
			c.Room.SetSong(data.Hash, data.Difficulty)
		case events.EVENT_USER_SONG_STATE:
			var data events.UserSongEventData
			err := json.Unmarshal(event.Data, &data)
			if err != nil {
				logger.Info("invalid client data", slog.Any("err", err))
				break
			}

			if !c.Room.IsInLobby() {
				break
			}

			if data.HasSong {
				c.SetNewState(CLIENT_IDLE)
			} else {
				c.SetNewState(CLIENT_MISSING_SONG)
			}

			c.Room.BroadcastAll(events.NewUserStateEvent(c.UUID, int(c.State)))
		case events.EVENT_USER_STATE:
			var data events.UserStateEventData
			err := json.Unmarshal(event.Data, &data)
			if err != nil {
				logger.Info("invalid client data", slog.Any("err", err))
				break
			}

			if c.State == CLIENT_MISSING_SONG {
				break
			}

			if data.State == 0 {
				c.SetNewState(CLIENT_IDLE)
			} else {
				c.SetNewState(CLIENT_LOBBY_READY)
			}
		case events.EVENT_ROOM_START:
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
		case events.EVENT_USER_READY:
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
		case events.EVENT_USER_SCORE:
			if !c.InMatch {
				break
			}
			if c.State != CLIENT_PLAYING {
				break
			}

			var data events.PartialGameplayScoreEventData
			err := json.Unmarshal(event.Data, &data)
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
				cl.Send <- events.NewGameplayScoreEvent(c.UUID, data.Score)
			}
		case events.EVENT_USER_FINISH:
			if !c.InMatch {
				break
			}
			if c.State != CLIENT_PLAYING {
				break
			}

			var data events.PartialGameplayFinishEventData
			err := json.Unmarshal(event.Data, &data)
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
					data.Score, events.JudgmentScore{
						Marvelous: data.Marvelous,
						Perfect:   data.Perfect,
						Great:     data.Great,
						Good:      data.Good,
						Boo:       data.Boo,
						Miss:      data.Miss,
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
