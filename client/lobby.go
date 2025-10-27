package main

import (
	"errors"
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
				panic("join:" + err.Error())
			}
			res, err := http.Get(p)
			if err != nil {
				if errors.Is(err, syscall.ECONNREFUSED) {
					i.Logger.Println("server is possibly inactive, exiting.")
				} else {
					i.Logger.Println("http error when polling lobby, exiting client...")
					i.Logger.Println("error:" + err.Error())
				}

				i.AttemptClose()
				return
			}

			d, err := io.ReadAll(res.Body)
			if err != nil {
				panic("read:" + err.Error())
			}

			buff, err := lemonade.EncodeStringToBuffer(string(d))
			if err != nil {
				panic("encode:" + err.Error())
			}

			buff = append([]int32{2, 1}, buff...)

			i.Lemon.WriteBuffer(buff)
		}

		time.Sleep(time.Second * 2)
	}
}
