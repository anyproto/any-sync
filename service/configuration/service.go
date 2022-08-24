package configuration

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/node"
	"github.com/anytypeio/go-chash"
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
	app.Component
}

type service struct {
	accountId string
	pool      pool.Pool

	last Configuration
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	conf := a.MustComponent(config.CName).(*config.Config)
	s.accountId = conf.Account.PeerId
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	configNodes := a.MustComponent(node.CName).(node.Service).Nodes()
	config := &configuration{
		id:        "config",
		accountId: s.accountId,
		pool:      s.pool,
	}
	if config.chash, err = chash.New(chash.Config{
		PartitionCount:    partitionCount,
		ReplicationFactor: replicationFactor,
	}); err != nil {
		return
	}
	members := make([]chash.Member, 0, len(configNodes))
	for _, n := range configNodes {
		members = append(members, n)
	}
	if err = config.chash.AddMembers(members...); err != nil {
		return
	}
	s.last = config
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) GetLast() Configuration {
	return s.last
}

func (s *service) GetById(id string) Configuration {
	//TODO implement me
	panic("implement me")
}
