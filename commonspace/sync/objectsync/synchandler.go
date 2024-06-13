package objectsync

import (
	"context"

	"github.com/cheggaaa/mb/v3"
	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
)

const CName = "common.sync.objectsync"

type objectSync struct {
	spaceId string
	manager treemanager.TreeManager
}

func (o *objectSync) Init(a *app.App) (err error) {
	o.manager = a.MustComponent(treemanager.CName).(treemanager.TreeManager)
	return
}

func (o *objectSync) Name() (name string) {
	return CName
}

func (o *objectSync) HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (syncdeps.Request, error) {
	//TODO implement me
	panic("implement me")
}

func (o *objectSync) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, sendResponse func(resp proto.Message) error) (syncdeps.Request, error) {
	//TODO implement me
	panic("implement me")
}

func (o *objectSync) ApplyRequest(ctx context.Context, rq syncdeps.Request, requestSender syncdeps.RequestSender) error {
	//TODO implement me
	panic("implement me")
}

func (o *objectSync) TryAddMessage(ctx context.Context, peerId string, msg drpc.Message, q *mb.MB[drpc.Message]) error {
	//TODO implement me
	panic("implement me")
}

func (o *objectSync) SendStreamRequest(ctx context.Context, rq syncdeps.Request, receive func(stream drpc.Stream) error) (err error) {
	//TODO implement me
	panic("implement me")
}

func (o *objectSync) NewResponse() syncdeps.Response {
	//TODO implement me
	panic("implement me")
}

func (o *objectSync) NewMessage() drpc.Message {
	//TODO implement me
	panic("implement me")
}
