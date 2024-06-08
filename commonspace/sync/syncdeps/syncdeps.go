package syncdeps

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
)

const CName = "common.sync.syncdeps"

type SyncHandler interface {
	app.Component
	HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (Request, error)
	TryAddMessage(ctx context.Context, msg drpc.Message, q *mb.MB[drpc.Message]) error
	HandleStreamRequest(ctx context.Context, rq Request, send func(resp proto.Message) error) (Request, error)
	SendStreamRequest(ctx context.Context, rq Request, receive func(stream drpc.Stream) error) (err error)
	HandleResponse(ctx context.Context, peerId, objectId string, resp Response) error
	NewResponse() Response
	NewMessage() drpc.Message
}
