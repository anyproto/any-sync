package syncqueues

import "strings"

func fullId(peerId, objectId string) string {
	return strings.Join([]string{peerId, objectId}, "-")
}
