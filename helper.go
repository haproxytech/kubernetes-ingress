package main

import (
	"log"
	"strings"

	"k8s.io/apimachinery/pkg/watch"
)

//LogWatchEvent log what kind of event occured
func LogWatchEvent(t watch.EventType, watchType SyncType, ObjData ...interface{}) {
	if t == watch.Added {
		log.Println(watchType, "added", ObjData)
	}
	if t == watch.Deleted {
		log.Println(watchType, "deleted", ObjData)
	}
	if t == watch.Modified {
		log.Println(watchType, "modified", ObjData)
	}
}

func hasSelectors(selectors map[string]string, values map[string]string) bool {
	for key, value1 := range selectors {
		value2, ok := values[key]
		if !ok {
			return false
		}
		if value1 != value2 {
			return false
		}
	}
	return true
}

func WriteBufferedString(builder *strings.Builder, data ...string) {
	for _, chunk := range data {
		builder.WriteString(chunk)
	}
}
