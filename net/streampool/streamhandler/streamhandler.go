package streamhandler

import (
	"context"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
)

const CName = "common.streampool.streamhandler"

// StreamHandler handles incoming messages from streams
type StreamHandler interface {
	app.Component
	// OpenStream opens stream with given peer
	OpenStream(ctx context.Context, p peer.Peer) (stream drpc.Stream, tags []string, err error)
	// HandleMessage handles incoming message
	HandleMessage(ctx context.Context, peerId string, msg drpc.Message) (err error)
	// NewReadMessage creates new empty message for unmarshalling into it
	NewReadMessage() drpc.Message
}
