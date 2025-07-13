package utils

import (
	"encoding/json"
	"os"
	"sync"
)

var hostInstance string
var hostOnce sync.Once

func GetHost() string {
	hostOnce.Do(func() {
		h, err := os.Hostname()
		if err != nil {
			hostInstance = "unknown"
		} else {
			hostInstance = h
		}
	})

	return hostInstance
}

func ToJSONString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "<marshal error>"
	}
	return string(b)
}
