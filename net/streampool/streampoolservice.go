package streampool

import (
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
)

const CName = "common.net.streampool"

var log = logger.NewNamed(CName)

func New() Service {
	return new(service)
}

type Service interface {
	NewStreamPool(h StreamHandler) StreamPool
	app.Component
}

type service struct {
}

func (s *service) NewStreamPool(h StreamHandler) StreamPool {
	return &streamPool{
		handler:         h,
		streamIdsByPeer: map[string][]uint32{},
		streamIdsByTag:  map[string][]uint32{},
		streams:         map[uint32]*stream{},
		opening:         map[string]chan struct{}{},
		exec:            newStreamSender(10, 100),
		lastStreamId:    0,
	}
}

func (s *service) Init(a *app.App) (err error) {
	return nil
}

func (s *service) Name() (name string) {
	return CName
}
