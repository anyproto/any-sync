package synchandler

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
)

type SyncHandler interface {
	HandleMessage(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (err error)
}
