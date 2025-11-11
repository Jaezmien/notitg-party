package main

import (
	"fmt"

	"git.jaezmien.com/Jaezmien/notitg-party/client/internal/utils"
	"gopkg.in/ini.v1"
)

func CheckBlacklist() (bool, error) {
	return utils.FileExists(BlacklistPath)
}

var (
	BlacklistedSongFiles   = make(map[string]bool, 0)
	BlacklistedSongFolders = make(map[string]bool, 0)
)

func CreateBlackistIni() {
	data := ini.Empty()

	section, err := data.NewSection("Blacklisted")
	if err != nil {
		panic(fmt.Errorf("ini section: %w", err))
	}

	_, err = section.NewKey("Folders", "")
	if err != nil {
		panic(fmt.Errorf("ini key: %w", err))
	}

	_, err = section.NewKey("Files", "")
	if err != nil {
		panic(fmt.Errorf("ini key: %w", err))
	}

	data.SaveTo(BlacklistPath)
}

func ReadBlacklist() {
	data, err := ini.Load(BlacklistPath)
	if err != nil {
		panic(fmt.Errorf("ini: %w", err))
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
