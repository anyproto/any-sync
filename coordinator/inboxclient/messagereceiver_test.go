package inboxclient

import "github.com/anyproto/any-sync/coordinator/coordinatorproto"

type MessageReceiverTest interface {
	Receive(event *coordinatorproto.InboxNotifySubscribeEvent)
}

type messageReceiverTest struct {
	mr MessageReceiver
}

func (f *messageReceiverTest) Receive(event *coordinatorproto.InboxNotifySubscribeEvent) {
	f.mr(event)
}
