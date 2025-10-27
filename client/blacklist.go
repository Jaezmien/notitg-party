package main

import (
	"errors"
	"log/slog"
	"os"

	"gopkg.in/ini.v1"
)

func CheckBlacklist() (bool, error) {
	_, err := os.Stat(BlacklistPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}

		return false, nil
	}
	return true, nil
}

var (
	BlacklistedSongFiles   = make(map[string]bool, 0)
	BlacklistedSongFolders = make(map[string]bool, 0)
)

func CreateBlackistIni() {
	data := ini.Empty()

	section, err := data.NewSection("Blacklisted")
	if err != nil {
		panic(err)
	}

	_, err = section.NewKey("Folders", "")
	if err != nil {
		panic(err)
	}

	_, err = section.NewKey("Files", "")
	if err != nil {
		panic(err)
	}

	data.SaveTo(BlacklistPath)
}

func ReadBlacklist() {
	data, err := ini.Load(BlacklistPath)
	if err != nil {
		slog.Error("ini error")
		panic(err)
	}
	section := data.Section("Blacklisted")

	songFiles := section.Key("Files").Strings(",")
	for _, songFile := range songFiles {
		BlacklistedSongFiles[songFile] = true
	}

	songFolders := section.Key("Folders").Strings(",")
	for _, songFolder := range songFolders {
		BlacklistedSongFolders[songFolder] = true
	}
}
