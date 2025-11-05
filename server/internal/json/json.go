package json

import "encoding/json"

func MustByte(data any) []byte {
	res, err := json.Marshal(data)
	if err != nil {
		panic("json:" + err.Error())
	}
	return res
}

func MustString(data any) string {
	return string(MustByte(data))
}

func MustUnmarshal(data []byte, v any) {
	err := json.Unmarshal(data, v)
	if err != nil {
		panic(err)
	}
}
