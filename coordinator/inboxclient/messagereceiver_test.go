//go:generate mockgen -source messagereceiver_test.go  -destination=mocks/mock_receiver.go -package=mocks github.com/anyproto/any-sync/coordinator/inboxclient MessageReceiverTest
package inboxclient

import "github.com/anyproto/any-sync/coordinator/coordinatorproto"

type MessageReceiverTest interface {
	Receive(event *coordinatorproto.NotifySubscribeEvent)
}
