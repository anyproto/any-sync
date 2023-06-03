package requestmanager

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

const CName = "common.commonspace.requestmanager"

var log = logger.NewNamed(CName)

type RequestManager interface {
	app.ComponentRunnable
	SendRequest(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error)
	QueueRequest(peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error)
}

func New() RequestManager {
	return &requestManager{}
}

type requestManager struct {
}

func (r *requestManager) Init(a *app.App) (err error) {
	return
}

func (r *requestManager) Name() (name string) {
	return CName
}

func (r *requestManager) Run(ctx context.Context) (err error) {
	return nil
}

func (r *requestManager) Close(ctx context.Context) (err error) {
	return nil
}

func (r *requestManager) SendRequest(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	return nil, nil
}

func (r *requestManager) QueueRequest(peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	return nil
}
