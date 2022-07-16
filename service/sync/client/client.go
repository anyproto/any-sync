package client

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/requesthandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/syncpb"
)

const CName = "SyncClient"

type client struct {
	handler requesthandler.RequestHandler
}

func NewClient() app.Component {
	return &client{}
}

type Client interface {
	NotifyHeadsChanged(update *syncpb.SyncHeadUpdate) error
	RequestFullSync(id string, request *syncpb.SyncFullRequest) error
	SendFullSyncResponse(id string, response *syncpb.SyncFullResponse) error
}

func (c *client) Init(ctx context.Context, a *app.App) (err error) {
	c.handler = a.MustComponent(requesthandler.CName).(requesthandler.RequestHandler)
	return nil
}

func (c *client) Name() (name string) {
	return CName
}

func (c *client) Run(ctx context.Context) (err error) {
	return nil
}

func (c *client) Close(ctx context.Context) (err error) {
	return nil
}

func (c *client) NotifyHeadsChanged(update *syncpb.SyncHeadUpdate) error {
	//TODO implement me
	panic("implement me")
}

func (c *client) RequestFullSync(id string, request *syncpb.SyncFullRequest) error {
	//TODO implement me
	panic("implement me")
}

func (c *client) SendFullSyncResponse(id string, response *syncpb.SyncFullResponse) error {
	//TODO implement me
	panic("implement me")
}
