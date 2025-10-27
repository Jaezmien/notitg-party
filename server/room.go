package main

import (
	"log/slog"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"git.jaezmien.com/Jaezmien/notitg-party/server/events"
)

type RoomState int

const (
	ROOM_IDLE RoomState = iota
	ROOM_PLAYING
)

type Room struct {
	UUID  string
	Title string

	Lobby *Lobby

	State RoomState

	SongHash       string
	SongDifficulty int

	Clients   map[*Client]bool
	Broadcast chan []byte
	Join      chan *Client
	Leave     chan *Client

	Quit chan struct{}
}

func (r *Room) SetNewState(state RoomState) {
	r.State = state
	r.BroadcastAll(events.NewRoomStateEvent(int(state)))
}

func (r *Room) ClientCount() int {
	return len(r.Clients)
}

func (r *Room) GetClientFromUsername(username string) *Client {
	for client := range r.Clients {
		if client.Username == username {
			return client
		}
	}

	return nil
}
func (r *Room) UsernameExists(username string) bool {
	client := r.GetClientFromUsername(username)
	return client != nil
}

func (r *Room) AllPlayersMissingSong() bool {
	for client := range r.Clients {
		if client.State != CLIENT_MISSING_SONG {
			return false
		}
	}

	return true
}

func (r *Room) IsReadyToStart() bool {
	if r.State != ROOM_IDLE {
		return false
	}

	if r.AllPlayersMissingSong() {
		return false
	}

	for client := range r.Clients {
		if client.State != CLIENT_LOBBY_READY && client.State != CLIENT_MISSING_SONG {
			return false
		}
	}

	return true
}

func (r *Room) IsReadyToPlay() bool {
	if r.State != ROOM_PLAYING {
		return false
	}

	for client := range r.Clients {
		if !client.InMatch {
			continue
		}

		if client.State != CLIENT_GAME_READY {
			return false
		}
	}

	return true
}

func (r *Room) IsInLobby() bool {
	return r.State == ROOM_IDLE
}
func (r *Room) IsPlaying() bool {
	return r.State == ROOM_PLAYING
}

func (r *Room) IsAllFinished() bool {
	if r.State != ROOM_PLAYING {
		return false
	}

	for client := range r.Clients {
		if !client.InMatch {
			continue
		}

		if client.State != CLIENT_RESULTS {
			return false
		}
	}
	return true
}

func (r *Room) Run() {
	logger.Info("new room created", slog.String("id", r.UUID))
	defer logger.Info("room has closed", slog.String("id", r.UUID))

	for {
		select {
		case <-r.Quit:
			return

		case client := <-r.Join:
			r.Clients[client] = true
			logger.Info("user has joined a room", slog.String("username", client.Username), slog.String("room id", r.UUID))

			// Send user's own data
			client.Send <- events.NewUserInfoEvent(client.Username, client.UUID)

			// Send room title
			client.Send <- events.NewRoomTitleEvent(r.Title)

			// Send room id
			client.Send <- events.NewRoomIDEvent(r.UUID)

			// Send room state
			client.Send <- events.NewRoomStateEvent(int(r.State))

			// Simulate the other players joining the room
			for cli := range r.Clients {
				client.Send <- events.NewUserJoinEvent(cli.Username, cli.UUID, int(cli.State))
			}

			// If there is only one user after joining, "reroll" the host
			if !r.HasHost() {
				r.RollNewHost()
			}
			if host := r.GetHost(); host != nil {
				client.Send <- events.NewRoomHostEvent(host.UUID)
			}

			if r.SongHash != "" {
				client.Send <- events.NewRoomSongEvent(r.SongHash, r.SongDifficulty)
			}

			// Send join event to the other clients
			r.BroadcastExcept(
				client.UUID,
				events.NewUserJoinEvent(client.Username, client.UUID, int(client.State)),
			)

		case client := <-r.Leave:
			if _, ok := r.Clients[client]; ok {
				r.CloseClient(client)
				logger.Info("user has left a room", slog.String("user", client.UUID), slog.String("room id", r.UUID))

				if r.ClientCount() <= 0 {
					logger.Info("all users have left a room, exiting room", slog.String("id", r.UUID))
					r.Lobby.CloseRoom(r.UUID)
				} else {
					r.BroadcastExcept(
						client.UUID,
						events.NewUserLeaveEvent(client.UUID),
					)

					if client.Host {
						logger.Info("host has left a room, selecting new host", slog.String("room id", r.UUID))
						r.RollNewHost()
						r.BroadcastHost()
					}
				}
			}

		case message := <-r.Broadcast:
			r.BroadcastAll(message)
		}
	}
}

func (r *Room) BroadcastAll(data []byte) {
	for cli := range r.Clients {
		select {
		case cli.Send <- data:
		default:
			r.CloseClient(cli)
		}
	}
}
func (r *Room) BroadcastExcept(clientID string, data []byte) {
	for cli := range r.Clients {
		if cli.UUID != clientID {
			select {
			case cli.Send <- data:
			default:
				r.CloseClient(cli)
			}
		}
	}
}

func (r *Room) HasHost() bool {
	for cli := range r.Clients {
		if cli.Host {
			return true
		}
	}
	return false
}
func (r *Room) RollNewHost() {
	// Reset all client's host status to false
	for cli := range r.Clients {
		cli.Host = false
	}

	// Pick the first client, and allow them to be host
	for cli := range r.Clients {
		logger.Info("a new host has been selected for a room", slog.String("user id", cli.UUID), slog.String("room id", r.UUID))
		cli.Host = true
		return
	}
}
func (r *Room) GetHost() *Client {
	for cli := range r.Clients {
		if cli.Host {
			return cli
		}
	}
	return nil
}
func (r *Room) BroadcastHost() {
	for cli := range r.Clients {
		if cli.Host {
			r.BroadcastAll(events.NewRoomHostEvent(cli.UUID))
			return
		}
	}
}

func (r *Room) SetSong(hash string, difficulty int) {
	r.SongHash = hash
	r.SongDifficulty = difficulty

	// Reset client states
	for cli := range r.Clients {
		cli.SetNewState(CLIENT_MISSING_SONG)
	}

	r.BroadcastAll(events.NewRoomSongEvent(hash, difficulty))
}

func (r *Room) Close() {
	logger.Info("closing room", slog.String("id", r.UUID))

	for c := range r.Clients {
		r.CloseClient(c)
	}

	close(r.Quit)
}

// --- //

func (r *Room) NewClient(c *websocket.Conn, name string) *Client {
	client := &Client{
		Connection: c,
		Username:   name,
		Room:       r,
		Send:       make(chan []byte, 256),
		UUID:       uuid.NewString(),
		Closed:     false,
		State:      CLIENT_IDLE,
	}

	if len(r.Clients) == 0 {
		client.Host = true
	}

	r.Join <- client
	return client
}
func (r *Room) CloseClient(c *Client) {
	c.Close()
	delete(r.Clients, c)
}
