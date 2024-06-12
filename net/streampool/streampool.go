package streampool

import (
	"context"

	"storj.io/drpc"

	peer2 "github.com/anyproto/any-sync/net/peer"
)

// StreamPool keeps and read streams
type StreamPool interface {
	// AddStream adds new outgoing stream into the pool
	AddStream(stream drpc.Stream, tags ...string) (err error)
	// ReadStream adds new incoming stream and synchronously read it
	ReadStream(stream drpc.Stream, tags ...string) (err error)
	// Send sends a message to given peers. A stream will be opened if it is not cached before. Works async.
	Send(ctx context.Context, msg drpc.Message, target PeerGetter) (err error)
	// SendById sends a message to given peerIds. Works only if stream exists
	SendById(ctx context.Context, msg drpc.Message, peerIds ...string) (err error)
	// Broadcast sends a message to all peers with given tags. Works async.
	Broadcast(ctx context.Context, msg drpc.Message, tags ...string) (err error)
	// AddTagsCtx adds tags to stream, stream will be extracted from ctx
	AddTagsCtx(ctx context.Context, tags ...string) error
	// RemoveTagsCtx removes tags from stream, stream will be extracted from ctx
	RemoveTagsCtx(ctx context.Context, tags ...string) error
	// Streams gets all streams for specific tags
	Streams(tags ...string) (streams []drpc.Stream)
	// Close closes all streams
	Close() error
}

// PeerGetter should dial or return cached peers
type PeerGetter func(ctx context.Context) (peers []peer2.Peer, err error)
