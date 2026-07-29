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

// CtxStreamId returns the id of the stream that delivered the current message.
// It is set on the context passed to StreamHandler.HandleMessage, letting a
// handler key per-stream state (e.g. subscription interest) so that two streams
// from the same peer stay independent.
func CtxStreamId(ctx context.Context) (streamId uint32, ok bool) {
	streamId, ok = ctx.Value(streamCtxKeyStreamId).(uint32)
	return
}
