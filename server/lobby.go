package main

import (
	"sync"

	"github.com/google/uuid"
	"github.com/sio/coolname"
)

type Lobby struct {
	RoomMutex sync.Mutex
	Roomes    map[*Room]bool
}

func NewLobby() *Lobby {
	return &Lobby{
		Roomes: make(map[*Room]bool),
	}
}

func CreateLobbyName() string {
	n, err := coolname.SlugN(3)
	if err != nil {
		panic(err)
	}
	return n
}

func (l *Lobby) NewRoom() *Room {
	m := &Room{
		UUID:  uuid.NewString(),
		Title: CreateLobbyName(),

		Lobby:    l,
		State:    ROOM_IDLE,
		SongHash: "",

		Broadcast: make(chan []byte),
		Clients:   make(map[*Client]bool),
		Join:      make(chan *Client),
		Leave:     make(chan *Client),

		Quit: make(chan struct{}),
	}

	l.RoomMutex.Lock()
	l.Roomes[m] = true
	l.RoomMutex.Unlock()

	go m.Run()

	return m
}

func (l *Lobby) GetRoom(id string) *Room {
	l.RoomMutex.Lock()
	defer l.RoomMutex.Unlock()

	for m := range l.Roomes {
		if m.UUID == id {
			return m
		}
	}

	return nil
}

func (l *Lobby) CloseRoom(id string) {
	l.RoomMutex.Lock()
	defer l.RoomMutex.Unlock()

	for m := range l.Roomes {
		if m.UUID == id {
			m.Close()
			delete(l.Roomes, m)
			return
		}
	}

	panic("attempted to close a room that doesn't exist")
}

func (l *Lobby) UsernameExists(username string) bool {
	l.RoomMutex.Lock()
	defer l.RoomMutex.Unlock()

	for m := range l.Roomes {
		if m.UsernameExists(username) {
			return true
		}
	}

	return false
}

func (l *Lobby) GetRoomCount() int {
	l.RoomMutex.Lock()
	defer l.RoomMutex.Unlock()

	return len(l.Roomes)
}

type RoomSummary struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Players []string  `json:"players"`
	State   RoomState `json:"state"`
}

func (l *Lobby) GetRoomSummary() []RoomSummary {
	l.RoomMutex.Lock()
	defer l.RoomMutex.Unlock()

	s := make([]RoomSummary, 0)
	for m := range l.Roomes {
		summary := RoomSummary{
			ID:      m.UUID,
			Title:   m.Title,
			State:   m.State,
			Players: make([]string, 0),
		}

		for p := range m.Clients {
			summary.Players = append(summary.Players, p.Username)
		}

		s = append(s, summary)
	}

	return s
}
