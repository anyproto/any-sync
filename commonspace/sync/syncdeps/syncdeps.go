package syncdeps

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/util/multiqueue"
)

const CName = "common.sync.syncdeps"

type ResponseCollector interface {
	NewResponse() Response
	CollectResponse(ctx context.Context, peerId, objectId string, resp Response) error
}

type RequestSender interface {
	SendRequest(ctx context.Context, rq Request, collector ResponseCollector) error
}

type ObjectSyncHandler interface {
	HandleHeadUpdate(ctx context.Context, statusUpdater syncstatus.StatusUpdater, headUpdate drpc.Message) (Request, error)
	HandleStreamRequest(ctx context.Context, rq Request, updater QueueSizeUpdater, send func(resp proto.Message) error) (Request, error)
	HandleResponse(ctx context.Context, peerId, objectId string, resp Response) error
	ResponseCollector() ResponseCollector
}

type QueueSizeUpdater interface {
	UpdateQueueSize(size uint64, msgType int, add bool)
}

type SyncHandler interface {
	app.Component
	HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (Request, error)
	HandleStreamRequest(ctx context.Context, rq Request, updater QueueSizeUpdater, sendResponse func(resp proto.Message) error) (Request, error)
	ApplyRequest(ctx context.Context, rq Request, requestSender RequestSender) error
	TryAddMessage(ctx context.Context, peerId string, msg multiqueue.Sizeable, q *mb.MB[multiqueue.Sizeable]) error
	SendStreamRequest(ctx context.Context, rq Request, receive func(stream drpc.Stream) error) (err error)
	NewMessage() drpc.Message
}
