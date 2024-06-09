package streamopener

import (
	"context"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
)

const CName = "common.streampool.streamopener"

// StreamOpener handles incoming messages from streams
type StreamOpener interface {
	app.Component
	// OpenStream opens stream with given peer
	OpenStream(ctx context.Context, p peer.Peer) (stream drpc.Stream, tags []string, err error)
}
