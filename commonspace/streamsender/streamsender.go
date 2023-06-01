package streamsender

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

const CName = "common.commonspace.streamsender"

type StreamSender interface {
	app.ComponentRunnable
	SendPeer(peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error)
	Broadcast(msg *spacesyncproto.ObjectSyncMessage) (err error)
}
