package synchandler

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
)

type SyncHandler interface {
	HandleMessage(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (err error)
}
