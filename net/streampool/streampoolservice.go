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

type StreamConfig struct {
	// SendQueueWorkers how many workers will write message to streams
	SendQueueWorkers int
	// SendQueueSize size of the queue for write
	SendQueueSize int
	// DialQueueWorkers how many workers will dial to peers
	DialQueueWorkers int
	// DialQueueSize size of the dial queue
	DialQueueSize int
}

type Service interface {
	NewStreamPool(h StreamHandler, conf StreamConfig) StreamPool
	app.Component
}

type service struct {
}

func (s *service) NewStreamPool(h StreamHandler, conf StreamConfig) StreamPool {
	sp := &streamPool{
		handler:         h,
		streamIdsByPeer: map[string][]uint32{},
		streamIdsByTag:  map[string][]uint32{},
		streams:         map[uint32]*stream{},
		opening:         map[string]*openingProcess{},
		exec:            newExecPool(conf.SendQueueWorkers, conf.SendQueueSize),
		dial:            newExecPool(conf.DialQueueWorkers, conf.DialQueueSize),
	}
	return sp
}

func (s *service) Init(a *app.App) (err error) {
	return nil
}

func (s *service) Name() (name string) {
	return CName
}
