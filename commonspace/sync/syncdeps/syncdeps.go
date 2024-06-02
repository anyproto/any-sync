package syncdeps

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"storj.io/drpc"
)

type MergeFilterFunc func(ctx context.Context, msg drpc.Message, q *mb.MB[drpc.Message]) error

type SyncDeps struct {
	HeadUpdateHandler      HeadUpdateHandler
	ResponseHandler        ResponseHandler
	RequestHandler         RequestHandler
	RequestSender          RequestSender
	MergeFilter            MergeFilterFunc
	ReadMessageConstructor func() drpc.Message
}
