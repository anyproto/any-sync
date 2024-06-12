package syncdeps

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
)

const CName = "common.sync.syncdeps"

type ObjectSyncHandler interface {
	HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (Request, error)
	HandleStreamRequest(ctx context.Context, rq Request, send func(resp proto.Message) error) (Request, error)
	HandleResponse(ctx context.Context, peerId, objectId string, resp Response) error
}

type SyncHandler interface {
	app.Component
	ObjectSyncHandler
	TryAddMessage(ctx context.Context, peerId string, msg drpc.Message, q *mb.MB[drpc.Message]) error
	SendStreamRequest(ctx context.Context, rq Request, receive func(stream drpc.Stream) error) (err error)
	NewResponse() Response
	NewMessage() drpc.Message
}
