package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"syscall"
	"time"

	lemonade "github.com/Jaezmien/notitg-lemonade-go"
)

func (i *LemonInstance) PollLobby() {
	for {
		if !i.IsInRoom() {
			p, err := url.JoinPath(Server, "/")
			if err != nil {
				panic(fmt.Errorf("join: %w", err))
			}
			res, err := http.Get(p)
			if err != nil {
				if errors.Is(err, syscall.ECONNREFUSED) {
					i.Logger.Info("server is possibly inactive, exiting.")
				} else {
					i.Logger.Debug("http error when polling lobby, exiting client...")
					i.Logger.Debug("error:" + err.Error())
				}

				i.AttemptClose()
				return
			}

			d, err := io.ReadAll(res.Body)
			if err != nil {
				panic(fmt.Errorf("read: %w", err))
			}

			buff, err := lemonade.EncodeStringToBuffer(string(d))
			if err != nil {
				panic(fmt.Errorf("encode: %w", err))
			}

			buff = append([]int32{2, 1}, buff...)

			i.Lemon.WriteBuffer(buff)
		}

		time.Sleep(time.Second * 2)
	}
}
