package streamsender

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

const CName = "common.commonspace.streamsender"

type StreamSender interface {
	app.Component
	SendPeer(peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error)
	Broadcast(msg *spacesyncproto.ObjectSyncMessage) (err error)
}

func New() StreamSender {
	return &streamSender{}
}

type streamSender struct {
}

func (s *streamSender) Init(a *app.App) (err error) {
	return
}

func (s *streamSender) Name() (name string) {
	return CName
}

func (s *streamSender) SendPeer(peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	return nil
}

func (s *streamSender) Broadcast(msg *spacesyncproto.ObjectSyncMessage) (err error) {
	return nil
}
