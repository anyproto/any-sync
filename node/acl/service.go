package acl

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusclient"
)

const CName = "node.acl"

type Service interface {
	app.ComponentRunnable
}

type aclService struct {
	cons   consensusclient.Service
	stream consensusclient.Stream
}

func (as *aclService) Init(a *app.App) (err error) {
	as.cons = a.MustComponent(consensusclient.CName).(consensusclient.Service)
	return nil
}

func (as *aclService) Name() (name string) {
	return CName
}

func (as *aclService) Run(ctx context.Context) (err error) {
	if as.stream, err = as.cons.WatchLog(ctx); err != nil {
		return
	}
	return nil
}

func (as *aclService) Close(ctx context.Context) (err error) {
	return as.stream.Close()
}
