package encoding

import (
	"storj.io/drpc"
)

func WrapHandler(h drpc.Handler) drpc.Handler {
	return &handleWrap{Handler: h}
}

type handleWrap struct {
	drpc.Handler
}

func (s *handleWrap) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	if CtxIsSnappy(stream.Context()) {
		stream = streamWrap{
			Stream:   stream,
			encoding: defaultSnappyEncoding,
		}
	} else {
		stream = streamWrap{
			Stream:   stream,
			encoding: defaultProtoEncoding,
		}
	}
	return s.Handler.HandleRPC(stream, rpc)
}
