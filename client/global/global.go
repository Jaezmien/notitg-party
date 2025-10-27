package global

import "encoding/json"

func JSONMustByte(data any) []byte {
	res, err := json.Marshal(data)
	if err != nil {
		panic("json:" + err.Error())
	}
	return res
}

func JSONMustString(data any) string {
	return string(JSONMustByte(data))
}
