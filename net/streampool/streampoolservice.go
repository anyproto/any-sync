package streampool

import (
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/metric"
)

const CName = "common.net.streampool"

var log = logger.NewNamed(CName)

func New() Service {
	return new(service)
}

type StreamConfig struct {
	// SendQueueSize size of the queue for write per peer
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
	metric metric.Metric
}

func (s *service) NewStreamPool(h StreamHandler, conf StreamConfig) StreamPool {
	sp := &streamPool{
		handler:         h,
		writeQueueSize:  conf.SendQueueSize,
		streamIdsByPeer: map[string][]uint32{},
		streamIdsByTag:  map[string][]uint32{},
		streams:         map[uint32]*stream{},
		opening:         map[string]*openingProcess{},
		dial:            newExecPool(conf.DialQueueWorkers, conf.DialQueueSize),
	}
	if s.metric != nil {
		registerMetrics(s.metric.Registry(), sp, "")
	}
	return sp
}

func (s *service) Init(a *app.App) (err error) {
	s.metric, _ = a.Component(metric.CName).(metric.Metric)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}
