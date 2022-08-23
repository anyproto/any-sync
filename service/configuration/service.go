package configuration

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/peer"
)

const CName = "configuration"

const (
	partitionCount    = 3000
	replicationFactor = 3
)

var log = logger.NewNamed(CName)

type Service interface {
	GetLast() Configuration
	GetById(id string) Configuration
	app.ComponentRunnable
}

type service struct {
	accountId       string
	bootstrapConfig []config.Node
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	conf := a.MustComponent(config.CName).(*config.Config)
	s.bootstrapConfig = conf.Nodes
	s.accountId = conf.Account.PeerId

	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) AllPeers(ctx context.Context, spaceId string) (peers []peer.Peer, err error) {
	//TODO implement me
	panic("implement me")
}

func (s *service) OnePeer(ctx context.Context, spaceId string) (p peer.Peer, err error) {
	//TODO implement me
	panic("implement me")
}

func (s *service) Run(ctx context.Context) (err error) {
	//TODO implement me
	panic("implement me")
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}
