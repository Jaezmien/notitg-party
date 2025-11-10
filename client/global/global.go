package global

import (
	"encoding/json"
	"fmt"
)

func JSONMustByte(data any) []byte {
	res, err := json.Marshal(data)
	if err != nil {
		panic(fmt.Errorf("json: %w", err))
	}
	return res
}

func JSONMustString(data any) string {
	return string(JSONMustByte(data))
}
