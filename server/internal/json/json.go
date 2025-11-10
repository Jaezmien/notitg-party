package json

import (
	"encoding/json"
	"fmt"
)

func MustByte(data any) []byte {
	res, err := json.Marshal(data)
	if err != nil {
		panic(fmt.Errorf("json: %w", err))
	}
	return res
}

func MustString(data any) string {
	return string(MustByte(data))
}

func MustUnmarshal(data []byte, v any) {
	err := json.Unmarshal(data, v)
	if err != nil {
		panic(fmt.Errorf("json: %w", err))
	}
}
