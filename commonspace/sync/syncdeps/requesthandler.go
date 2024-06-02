package syncdeps

import (
	"context"

	"github.com/gogo/protobuf/proto"
)

type RequestHandler interface {
	HandleStreamRequest(ctx context.Context, rq Request, send func(resp proto.Message) error) (Request, error)
}
