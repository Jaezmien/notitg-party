package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"git.jaezmien.com/Jaezmien/notitg-party/server/events"
	"github.com/gorilla/websocket"
)

type ClientState int

var ClientScoreThrottleMS = time.Duration(time.Second * 1).Milliseconds()

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

	UserScoreThrottle int64
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
			logger.Debug("error while parsing client message", slog.Any("err", err))
			break
		}

		if event.Type != events.EVENT_USER_SCORE {
			logger.Debug(fmt.Sprintf("received event: %s", event.Type))
		}

		switch event.Type {
		case events.EVENT_ROOM_SONG:
			var data events.SongEventData
			err := json.Unmarshal(event.Data, &data)
			if err != nil {
				logger.Debug("invalid client data", slog.Any("err", err))
				break
			}

			if !c.Room.IsIdle() {
				logger.Debug("room not in lobby, ignoring song change")
				break
			}
			if !c.Host {
				logger.Debug("client is not host, ignoring")
				break
			}

			logger.Info("changing song!")
			c.Room.SetSong(data.Hash, data.Difficulty)
		case events.EVENT_USER_SONG_STATE:
			var data events.UserSongEventData
			err := json.Unmarshal(event.Data, &data)
			if err != nil {
				logger.Debug("invalid client data", slog.Any("err", err))
				break
			}

			if !c.Room.IsIdle() {
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
				logger.Debug("invalid client data", slog.Any("err", err))
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

			c.Room.ReadyMatch()
		case events.EVENT_USER_READY:
			if c.State != CLIENT_GAME_LOADING {
				break
			}

			c.SetNewState(CLIENT_GAME_READY)

			c.Room.StartMatch(false)
		case events.EVENT_USER_SCORE:
			if !c.InMatch {
				break
			}
			if c.State != CLIENT_PLAYING {
				break
			}

			// Throttle client score events (considering that we're constantly broadcasting this)
			now := time.Now().UnixMilli()
			if now < c.UserScoreThrottle {
				break
			}
			c.UserScoreThrottle = now + ClientScoreThrottleMS

			var data events.PartialGameplayScoreEventData
			err := json.Unmarshal(event.Data, &data)
			if err != nil {
				logger.Debug("invalid client data", slog.Any("err", err))
				break
			}

			c.Room.ForClientInMatch(func(cl *Client) {
				if cl.UUID == c.UUID {
					return
				}

				cl.Send <- events.NewGameplayScoreEvent(c.UUID, data.Score)
			})
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
				logger.Debug("invalid client data", slog.Any("err", err))
				break
			}

			c.Room.ForClientInMatch(func(cl *Client) {
				if cl.UUID == c.UUID {
					return
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
			})

			c.SetNewState(CLIENT_RESULTS)
			c.Room.BroadcastAll(events.NewRoomStateEvent(int(CLIENT_RESULTS)))

			// We're the host, we're the source of truth.
			// If we have finished, then we can tell the server that the end time has been reached.
			if c.Host {
				c.Room.UpdateExpectedMatchEnd()
			}

			logger.Info("player has finished song", slog.String("id", c.Room.UUID))

			c.Room.FinishMatch(false)
		}
	}

	c.Room.Leave <- c
}
