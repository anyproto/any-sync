package peerstatus

import (
	"github.com/anyproto/any-sync/app"
)

type Status int32

const (
	Unknown      Status = 0
	Connected    Status = 1
	NotPossible  Status = 2
	NotConnected Status = 3
)

type StatusUpdateSender interface {
	app.ComponentRunnable
	SendPeerUpdate()
	SendNewStatus(status Status)
}
