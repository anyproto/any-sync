package message

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/syncpb"
)

const CName = "Service"

type service struct {
}

func NewMessageService() app.Component {
	return &service{}
}

type Service interface {
	RegisterMessageSender(peerId string) chan *syncpb.SyncContent
	UnregisterMessageSender(peerId string)
	HandleMessage(peerId string, msg *syncpb.SyncContent)
}

func (c *service) Init(ctx context.Context, a *app.App) (err error) {
	return nil
}

func (c *service) Name() (name string) {
	return CName
}

func (c *service) Run(ctx context.Context) (err error) {
	return nil
}

func (c *service) Close(ctx context.Context) (err error) {
	return nil
}

func (c *service) RegisterMessageSender(peerId string) chan *syncpb.SyncContent {
	//TODO implement me
	panic("implement me")
}

func (c *service) UnregisterMessageSender(peerId string) chan *syncpb.SyncContent {
	//TODO implement me
	panic("implement me")
}

func (c *service) HandleMessage(peerId string, msg *syncpb.SyncContent) {
	//TODO implement me
	panic("implement me")
}
