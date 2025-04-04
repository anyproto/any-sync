package inboxclient

import "github.com/anyproto/any-sync/coordinator/coordinatorproto"

type MessageReceiverTest interface {
	Receive(event *coordinatorproto.InboxNotifySubscribeEvent)
}
