package synchandler

import (
	"context"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type SyncHandler interface {
	HandleMessage(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error)
	HandleRequest(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (response *spacesyncproto.ObjectSyncMessage, err error)
}
