package streampool

import (
	"context"

	peer2 "github.com/anyproto/any-sync/net/peer"
)

type streamCtxKey uint

const (
	streamCtxKeyStreamId streamCtxKey = iota
)

func streamCtx(ctx context.Context, streamId uint32, peerId string) context.Context {
	ctx = peer2.CtxWithPeerId(ctx, peerId)
	return context.WithValue(ctx, streamCtxKeyStreamId, streamId)
}
