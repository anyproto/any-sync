package rpcstore

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
)

const CName = "common.commonfile.rpcstore"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type Service interface {
	NewStore() fileblockstore.BlockStore
	app.Component
}

type service struct {
	pool     pool.Pool
	nodeconf nodeconf.Service
}

func (s *service) Init(a *app.App) (err error) {
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.nodeconf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) NewStore() fileblockstore.BlockStore {
	cm := newClientManager(s)
	return &store{
		s:  s,
		cm: cm,
	}
}

func (s *service) filePeers() []string {
	return s.nodeconf.GetLast().FilePeers()
}
