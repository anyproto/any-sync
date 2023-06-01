package requestsender

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

const CName = "common.commonspace.requestsender"

var log = logger.NewNamed(CName)

type RequestSender interface {
	app.ComponentRunnable
	SendRequest(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error)
	QueueRequest(peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error)
}

type requestSender struct {
}

func (r *requestSender) Init(a *app.App) (err error) {
	return
}

func (r *requestSender) Name() (name string) {
	return CName
}

func (r *requestSender) Run(ctx context.Context) (err error) {
	return nil
}

func (r *requestSender) Close(ctx context.Context) (err error) {
	return nil
}

func (r *requestSender) SendRequest(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	return nil, nil
}

func (r *requestSender) QueueRequest(peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	return nil
}
