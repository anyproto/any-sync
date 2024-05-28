package peerstatus

import (
	"github.com/anyproto/any-sync/app"
)

type StatusUpdateSender interface {
	app.ComponentRunnable
	CheckPeerStatus()
	SendNotPossibleStatus()
}
