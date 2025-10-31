package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

var Port int = 8080
var upgrader = websocket.Upgrader{}

var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

func init() {
	flag.IntVar(&Port, "port", 8080, "Sets the server port")

	flag.Parse()
}

func main() {
	logger.Info("initializing party...")

	lobby := NewLobby()

	http.HandleFunc("/room/join", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(400)
			fmt.Fprintf(w, "unknown method")
			return
		}

		q, _ := url.ParseQuery(r.URL.RawQuery)

		username := strings.TrimSpace(q.Get("username"))
		if username == "" {
			w.WriteHeader(400)
			fmt.Fprintf(w, "missing username")
			return
		}
		if lobby.UsernameExists(username) {
			w.WriteHeader(400)
			fmt.Fprintf(w, "username already exists")
			return
		}

		roomID := strings.TrimSpace(q.Get("room"))
		if roomID == "" {
			w.WriteHeader(400)
			fmt.Fprintf(w, "missing room")
			return
		}
		room := lobby.GetRoom(roomID)
		if room == nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "unknown room")
			return
		}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("error in upgrading connection", slog.Any("err", err))
			return
		}

		cl := room.NewClient(c, username)
		go cl.Write()
		go cl.Read()
	})
	http.HandleFunc("/room/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(400)
			fmt.Fprintf(w, "unknown method")
			return
		}

		room := lobby.NewRoom()

		data, err := json.Marshal(struct {
			ID string
		}{
			ID: room.UUID,
		})
		if err != nil {
			logger.Error("marshal error:", slog.Any("error", err))

			w.WriteHeader(500)
			fmt.Fprintf(w, "internal error")
			return
		}
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(400)
			fmt.Fprintf(w, "unknown method")
			return
		}

		summary := lobby.GetRoomSummary()

		data, err := json.MarshalIndent(summary, "", "\t")
		if err != nil {
			logger.Error("marshal error:", slog.Any("error", err))

			w.WriteHeader(500)
			fmt.Fprintf(w, "internal error")
			return
		}
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	logger.Info("ready to party!")
	err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", Port), nil)
	if err != nil {
		logger.Error("http:", slog.Any("err", err))
	}
}
