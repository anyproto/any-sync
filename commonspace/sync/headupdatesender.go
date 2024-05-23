package sync

import "context"

type BroadcastOptions struct {
	EmptyPeers []string
}

type HeadUpdateSender interface {
	SendHeadUpdate(ctx context.Context, peerId string, headUpdate *HeadUpdate) error
	BroadcastHeadUpdate(ctx context.Context, opts BroadcastOptions, headUpdate *HeadUpdate) error
}
