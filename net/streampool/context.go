package streampool

import (
	"context"
	"github.com/anyproto/any-sync/net/peer"
)

type streamCtxKey uint

const (
	streamCtxKeyStreamId streamCtxKey = iota
)

func streamCtx(ctx context.Context, streamId uint32, peerId string) context.Context {
	ctx = peer.CtxWithPeerId(ctx, peerId)
	return context.WithValue(ctx, streamCtxKeyStreamId, streamId)
}
