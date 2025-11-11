package utils

import (
	"errors"
	"os"
)

func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}

		return false, nil
	}
	return true, nil
}
