package utils

import (
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
