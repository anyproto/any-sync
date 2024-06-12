package streampool

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/debugstat"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/metric"

	streampool2 "github.com/anyproto/any-sync/net/streampool"
)

const CName = "common.net.streampool"

var log = logger.NewNamed(CName)

func New() Service {
	return new(service)
}

type Service interface {
	NewStreamPool(h StreamHandler, conf streampool2.StreamConfig) streampool2.StreamPool
	app.Component
}

type service struct {
	metric    metric.Metric
	debugStat debugstat.StatService
}

func (s *service) NewStreamPool(h StreamHandler, conf streampool2.StreamConfig) streampool2.StreamPool {
	pl := streampool2.NewExecPool(conf.DialQueueWorkers, conf.DialQueueSize)
	sp := &streamPool{
		handler:         h,
		writeQueueSize:  conf.SendQueueSize,
		streamIdsByPeer: map[string][]uint32{},
		streamIdsByTag:  map[string][]uint32{},
		streams:         map[uint32]*stream{},
		opening:         map[string]*openingProcess{},
		dial:            pl,
		statService:     s.debugStat,
	}
	sp.statService.AddProvider(sp)
	pl.Run()
	if s.metric != nil {
		registerMetrics(s.metric.Registry(), sp, "")
	}
	return sp
}

func (s *service) Init(a *app.App) (err error) {
	s.metric, _ = a.Component(metric.CName).(metric.Metric)
	s.debugStat, _ = a.Component(debugstat.CName).(debugstat.StatService)
	if s.debugStat == nil {
		s.debugStat = debugstat.NewNoOp()
	}
	return nil
}

func (s *service) Name() (name string) {
	return CName
}
