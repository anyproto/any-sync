package handler

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var log = logger.NewNamed("replyHandler")

type ReplyHandler interface {
	Handle(ctx context.Context, req []byte) (rep proto.Marshaler, err error)
}

type Reply struct {
	ReplyHandler
}

func (r Reply) Handle(ctx context.Context, msg *pool.Message) error {
	rep, e := r.ReplyHandler.Handle(ctx, msg.GetData())
	if msg.GetHeader().RequestId == 0 {
		if e != nil {
			log.Error("handler returned error", zap.Error(e))
		} else if rep != nil {
			log.Debug("sender didn't expect a reply, but the handler made")
		}
		return nil
	}
	return msg.ReplyType(msg.GetHeader().GetType(), rep)
}
