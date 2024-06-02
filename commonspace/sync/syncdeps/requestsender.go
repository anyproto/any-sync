package syncdeps

import (
	"context"

	"storj.io/drpc"
)

type RequestSender interface {
	SendStreamRequest(ctx context.Context, rq Request, receive func(stream drpc.Stream) error) (err error)
}
