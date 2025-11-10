package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"git.jaezmien.com/Jaezmien/notitg-party/client/events"
	"github.com/Jaezmien/notitg-lemonade-go"
	"github.com/gorilla/websocket"
	bolt "go.etcd.io/bbolt"
)

const (
	AppID = 2
)

var DeepScan = false
var ProcessID = 0
var Verbose = false
var Version = false

var Server = ""
var Username = ""

var SongsPath = ""
var HashDBPath = ""
var BlacklistPath = ""

type ClientState int

const (
	CLIENT_UNKNOWN ClientState = iota
	CLIENT_LOBBY
	CLIENT_ROOM
	CLIENT_GAME
	CLIENT_RESULT
)

var BuildVersion = "0.0.0-dev"
var BuildCommit = "dev"

func init() {
	flag.BoolVar(&DeepScan, "deep", false, "Scan deeply by checking each process' memory")
	flag.IntVar(&ProcessID, "pid", 0, "Use a specific process")
	flag.BoolVar(&Verbose, "verbose", false, "Enable debug messages")
	flag.StringVar(&SongsPath, "hash", "", "When provided with the directory to 'Songs/', will scan every song in the folder")
	flag.BoolVar(&Version, "version", false, "Display version info")

	flag.StringVar(&Server, "server", "http://localhost:8080", "The server to connect to")
	flag.StringVar(&Username, "username", "", "Your username")

	flag.Parse()

	if Version {
		fmt.Printf("notitg-party-client v%s@%s\n", BuildVersion, BuildCommit)
		os.Exit(0)
	}
}

func init() {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("error with os: %w", err))
	}
	BlacklistPath = filepath.Join(wd, "blacklist.ini")

	for {
		ok, err := CheckBlacklist()

		if err != nil {
			panic(fmt.Errorf("error with os: %w", err))
		}

		if ok {
			break
		}

		fmt.Println("[Blacklist] It seems like blacklist.ini is missing!")

		CreateBlackistIni()

		fmt.Println("[Blacklist] Created the default blacklist.ini file!")
		fmt.Println("[Blacklist] Please configure the blacklist.ini to your liking, then press Enter to continue.")

		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}

	ReadBlacklist()
}

func init() {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("os getwd: %w", err))
	}
	HashDBPath = filepath.Join(wd, "cache.db")

	db, err := bolt.Open(HashDBPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		panic(fmt.Errorf("db: %w", err))
	}
	defer db.Close()

	if err := CreateHashBucket(db, SongsPath != ""); err != nil {
		panic(fmt.Errorf("db: %w", err))
	}

	if SongsPath != "" {
		if err := ScanSongFolder(db, SongsPath); err != nil {
			panic(fmt.Errorf("song folder scan: %w", err))
		}

		os.Exit(0)
	} else {
		count := CountSongs(db)
		if count == 0 {
			panic("counted 0 songs! make sure you have ran -scan")
		}
	}
}

func init() {
	if strings.TrimSpace(Username) == "" {
		fmt.Println("username is required!")
		fmt.Println("provide it using `--username`")
		os.Exit(1)
	}

	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		fmt.Println("this program needs to run as root!")
		fmt.Println("use `sudo` or other alternatives.")
		os.Exit(1)
	}
}

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
		}

		data, err := io.ReadAll(t.Body)
		if err != nil {
			panic(fmt.Errorf("io read: %w", err))
		}
		i.Logger.Debug(string(data))
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

func main() {
	instance := NewLemonInstance(AppID)
	defer instance.Close()

	// XXX: oh boy i hope this doesn't catch on fire
	// you can tell how confident i am with goroutines :)
	go instance.PollLobby()

	db, err := bolt.Open(HashDBPath, 0600, nil)
	if err != nil {
		panic(fmt.Errorf("db: %w", err))
	}
	defer db.Close()

	instance.Lemon.OnConnect = func(l *lemonade.Lemonade) {
		instance.Logger.Info("Detected NotITG!", slog.Int("buildDate", l.NotITG.GetDetail().BuildDate))

		// Always assume that NotITG's state is unknown
		instance.State = CLIENT_UNKNOWN

		// Notify NotITG that we have detected it
		l.WriteBuffer([]int32{1, 1})
	}
	instance.Lemon.OnDisconnect = func(l *lemonade.Lemonade) {
		if instance.IsInRoom() {
			// Notify the server that our client has disconnected in the room.
			// This can be done by just closing the room websocket.
			instance.LeaveRoom()
		}
		instance.State = CLIENT_UNKNOWN

		if ProcessID != 0 {
			instance.Logger.Info("Detected NotITG using PID scan had exited, closing!")
			instance.Close()
		}
	}

	instance.Lemon.OnBufferRead = func(l *lemonade.Lemonade, buffer []int32) {
		instance.Logger.Debug("received buffer", slog.String("buffer", fmt.Sprintf("%v", buffer)))

		// Miscellaneous
		if buffer[0] == 1 {
			if buffer[1] == 1 {
				// Scenario: The user probably wants to exit the lobby - let's set the state to unknown!
				instance.State = CLIENT_UNKNOWN
				return
			}
			if buffer[1] == 2 {
				// Scenario: We're exiting, we've notified NotITG, and NotITG has acknowledged it.
				// We can now properly close!
				instance.Close()
				return
			}
		}

		// Lobby!
		if buffer[0] == 2 {
			if buffer[1] == 1 {
				// Scenario: NotITG has reported that it's on the lobby screen.
				if instance.State == CLIENT_LOBBY {
					return
				}

				if instance.IsInRoom() {
					instance.LeaveRoom()
				}

				if instance.IsActivePlaying() {
					// Hold up! We're supposed to reach the Evaluation Screen first before we get
					// to the lobby! This likely means that the player as quit from the
					// Gameplay screen back to the Lobby! DQ!

					// TODO: Disqualify player
				}

				instance.State = CLIENT_LOBBY
			}
			if buffer[1] == 2 {
				// Scenario: NotITG wants to create its own room
				if instance.IsInRoom() {
					return
				}
				id := instance.CreateRoom()
				instance.JoinRoom(id)
				return
			}
			if buffer[1] == 3 {
				// Scenario: NotITG wants to join an existing room.
				if instance.IsInRoom() {
					return
				}

				// The following buffer content is the room UUID
				uuid, err := lemonade.DecodeBufferToString(buffer[2:])
				if err != nil {
					panic(fmt.Errorf("decode: %w", err))
				}
				instance.JoinRoom(uuid)
				return
			}
		}

		// Room!
		if buffer[0] == 3 {
			if buffer[1] == 1 {
				// Scenario: NotITG has reported that it's on the room screen

				instance.State = CLIENT_ROOM
			}
			if buffer[1] >= 2 && !instance.IsInRoom() {
				panic("received room data while not in room")
			}
			if buffer[1] == 2 {
				// Scenario: (If host), NotITG wants to set a new song

				message, err := lemonade.DecodeBufferToString(buffer[2:])
				if err != nil {
					panic(fmt.Errorf("decode: %w", err))
				}

				// Attempt to read json data
				var songData struct {
					Key        string `json:"key"`
					Difficulty int    `json:"difficulty"`
				}

				if err := json.Unmarshal([]byte(message), &songData); err != nil {
					instance.Logger.Debug("error while parsing client message", "error", err)
					instance.AttemptClose()
					return
				}

				// Get hash of song
				if !HasSongKey(db, songData.Key) {
					instance.Logger.Info(fmt.Sprintf("client has no hash of this song! (%s)\n", songData.Key))
					instance.Logger.Info("run this program again with -scan")
					instance.AttemptClose()
					return
				}
				hash, has := GetSongHash(db, songData.Key)
				if !has {
					instance.Logger.Info(fmt.Sprintf("could not find song with key: %s\n", songData.Key))
					instance.AttemptClose()
					return
				}

				instance.Room.Send <- events.NewSetSongEvent(hash, songData.Difficulty)
			}
			if buffer[1] == 3 {
				// Scenario: NotITG received a song hash, and it wants us to verify if we have it

				hash, err := lemonade.DecodeBufferToString(buffer[2:])
				if err != nil {
					panic(fmt.Errorf("decode: %w", err))
				}

				// Verify it, and whatever the result is, send it to the server.
				has := HasSongHash(db, hash)
				instance.Room.Send <- events.NewUserSongEvent(has)

				// Don't have song? Just notify NotITG
				if !has {
					l.WriteBuffer([]int32{3, 1, 1})
					return
				}

				// If we have it, report back to NotITG on what song it should load, and its difficulty
				song, ok := GetSongKey(db, hash)
				if !ok {
					instance.Logger.Info(fmt.Sprintf("could not find song from hash: %s\n", hash))
					instance.AttemptClose()
					return
				}

				// We have a song key! Let's send it to NotITG.
				data, err := lemonade.EncodeStringToBuffer(song)
				if err != nil {
					panic(fmt.Errorf("encode: %w", err))
				}

				l.WriteBuffer(append([]int32{3, 1, 2}, data...))
			}
			if buffer[1] == 4 {
				// Scenario: NotITG wants us to set the client's room state

				if state := buffer[2]; state == 0 {
					// Set state to idle
					instance.Room.Send <- events.NewUserStateEvent(0)
				} else {
					// Set state to ready
					instance.Room.Send <- events.NewUserStateEvent(1)
				}
			}
			if buffer[1] == 5 {
				instance.Room.Send <- events.NewHostStartEvent()
			}
		}
		if buffer[0] == 4 {
			if buffer[1] == 1 {
				// Scenario: We're in ScreenGameplay, and NotITG is ready!
				instance.State = CLIENT_GAME
				instance.Room.Send <- events.NewGameplayReadyEvent()
			}
			if buffer[1] == 2 {
				// Scenario: Updating scores in real time!
				if instance.State != CLIENT_GAME {
					return
				}

				instance.Room.Send <- events.NewGameplayScoreEvent(buffer[2])
			}
			if buffer[1] == 3 {
				// Scenario: We have finished the song! Let's notify the server.
				if instance.State != CLIENT_GAME {
					return
				}

				score := buffer[2]
				marvelous := buffer[3]
				perfect := buffer[4]
				great := buffer[5]
				good := buffer[6]
				boo := buffer[7]
				miss := buffer[8]

				instance.Room.Send <- events.NewGameplayFinishEvent(score, events.JudgmentScore{
					Marvelous: marvelous,
					Perfect:   perfect,
					Great:     great,
					Good:      good,
					Boo:       boo,
					Miss:      miss,
				})

				instance.State = CLIENT_RESULT
			}
		}
	}

	// TODO: Unfuck whatever the fuck's happening down here

	// Let's catch SIGTERM and interrupt here
	termChannel := make(chan os.Signal, 2)
	signal.Notify(termChannel, os.Interrupt)
	go func() {
		<-termChannel
		instance.Logger.Debug("interrupt caught")
		instance.AttemptClose()
		<-termChannel
		instance.Logger.Info("very bad, will immediately exit")
		os.Exit(1)
	}()

	bufio.NewReader(os.Stdin).ReadBytes('\n')

	// hmm
	instance.AttemptClose()

	select {}
}
