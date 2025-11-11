package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"syscall"

	lemonade "github.com/Jaezmien/notitg-lemonade-go"
	"github.com/gorilla/websocket"
)

type LemonInstance struct {
	Lemon *lemonade.Lemonade

	Logger *slog.Logger

	State ClientState

	Room *RoomConnection

	Closing bool
}

func NewLemonInstance(appID int32) *LemonInstance {
	lemon, err := lemonade.New(appID, &lemonade.LemonadeInstanceConfig{
		DeepScan:  DeepScan,
		ProcessID: ProcessID,
		TickRate:  10,
	})
	if err != nil {
		panic(fmt.Errorf("lemonade: %w", err))
	}

	instance := &LemonInstance{
		Lemon: lemon,
		State: CLIENT_UNKNOWN,
	}

	if !Verbose {
		lemon.Logger.SetOutput(io.Discard)
	}

	slogOptions := &slog.HandlerOptions{}
	slogLevel := new(slog.LevelVar)
	if Verbose {
		slogLevel.Set(slog.LevelDebug)
	} else {
		slogLevel.Set(slog.LevelInfo)
	}
	slogOptions.Level = slogLevel

	instance.Logger = slog.New(slog.NewTextHandler(os.Stdout, slogOptions))

	return instance
}

func (i *LemonInstance) AttemptClose() {
	if i.Closing {
		return
	}
	i.Closing = true

	if i.Lemon.NotITG == nil {
		// Well, we don't have NotITG detected, let's just close as is.
		i.Close()
	}

	i.Logger.Debug("attempting to close properly...")
	i.Lemon.WriteBuffer([]int32{1, 2})
}

func (i *LemonInstance) Close() {
	i.Lemon.Close()
	os.Exit(0)
}

func (i *LemonInstance) SendString(data string, prefix []int32) {
	buff, err := lemonade.EncodeStringToBuffer(data)
	if err != nil {
		panic(fmt.Errorf("encode: %w", err))
	}

	i.Lemon.WriteBuffer(append(prefix, buff...))
}

func (i *LemonInstance) JoinRoom(id string) *websocket.Conn {
	re := regexp.MustCompile("https?://")
	s := re.ReplaceAllString(Server, "")

	scheme := "ws"
	if strings.HasPrefix(Server, "https") {
		scheme = "wss"
	}

	u := url.URL{Scheme: scheme, Host: s, Path: "/room/join"}
	q := u.Query()
	q.Add("username", Username)
	q.Add("room", id)
	u.RawQuery = q.Encode()

	c, t, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) {
			fmt.Println("server is possibly inactive, exiting.")
			i.AttemptClose()
			return nil
		}

		if t != nil {
			data, err := io.ReadAll(t.Body)
			if err != nil {
				panic(fmt.Errorf("io read: %w", err))
			}
			i.Logger.Debug(string(data))
		}
		return nil
	}

	i.Room = NewRoomConnection(c, i)
	go i.Room.Read()
	go i.Room.Write()

	i.Lemon.WriteBuffer([]int32{2, 2}) // Send to NotITG that we're in a room

	return c
}
func (i *LemonInstance) CreateRoom() string {
	p, err := url.JoinPath(Server, "/room/create")
	if err != nil {
		panic(fmt.Errorf("join: %w", err))
	}
	res, err := http.Post(p, "", nil)
	if err != nil {
		panic(fmt.Errorf("http post: %w", err))
	}

	var data struct{ ID string }
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		panic(fmt.Errorf("json: %w", err))
	}

	return data.ID
}

func (i *LemonInstance) IsInLobby() bool {
	return i.State == CLIENT_LOBBY
}
func (i *LemonInstance) IsInRoom() bool {
	if i.Room == nil {
		return false
	}

	if i.State == CLIENT_UNKNOWN {
		return false
	}
	if i.State == CLIENT_LOBBY {
		return false
	}

	return true
}
func (i *LemonInstance) IsPlaying() bool {
	if i.Room == nil {
		return false
	}
	if !i.IsInRoom() {
		return false
	}

	return i.State == CLIENT_GAME || i.State == CLIENT_RESULT
}
func (i *LemonInstance) IsActivePlaying() bool {
	return i.Room != nil && (i.State == CLIENT_GAME)
}

func (i *LemonInstance) LeaveRoom() {
	if !i.IsInRoom() {
		panic("attempted to leave room while not in a room state")
	}

	// XXX: We need to state that it's closed, before closing the connection
	// Otherwise, ReadMessage attempts to read one last message, and hits Fatal
	i.Room.Closed = true
	i.Room.Connection.Close()
	i.Room = nil
}
