package main

import (
	"encoding/json"
	"log/slog"

	"github.com/gorilla/websocket"
)

type RoomConnection struct {
	Connection *websocket.Conn
	Instance   *LemonInstance
	Closed     bool

	Send chan []byte
}

func NewRoomConnection(con *websocket.Conn, instance *LemonInstance) *RoomConnection {
	return &RoomConnection{
		Connection: con,
		Instance:   instance,
		Closed:     false,
		Send:       make(chan []byte),
	}
}

func (m *RoomConnection) Read() {
	for {
		t, message, err := m.Connection.ReadMessage()

		if m.Closed {
			return
		}

		if err != nil {
			if _, ok := err.(*websocket.CloseError); ok {
				m.Instance.Logger.Debug("websocket close error", slog.Any("error", err))
			} else {
				m.Instance.Logger.Debug("websocket read error", slog.Any("error", err))
			}
			m.Connection.Close()
			m.Instance.AttemptClose()
			return
		}

		if t != websocket.TextMessage {
			m.Instance.Logger.Debug("unknown server message type")
			continue
		}

		// Validate json
		var j json.RawMessage
		if err := json.Unmarshal(message, &j); err != nil {
			m.Instance.Logger.Warn("invalid server message", slog.String("message", string(message)))
			continue
		}

		m.Instance.SendString(string(message), []int32{99})
	}
}

func (m *RoomConnection) Write() {
	for message := range m.Send {
		w, err := m.Connection.NextWriter(websocket.TextMessage)
		if err != nil {
			panic(err)
		}
		w.Write(message)

		if err := w.Close(); err != nil {
			panic(err)
		}
	}
}
