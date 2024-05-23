package sync

import (
	"context"

	"storj.io/drpc"
)

type HeadUpdateHandler interface {
	HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (Request, error)
}
