package consensusclient

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	"go.uber.org/zap"
	"math/rand"
	"sync"
)

const CName = "consensus.client"

var log = logger.NewNamed(CName)

func New() Service {
	return new(service)
}

type Service interface {
	AddLog(ctx context.Context, clog *consensusproto.Log) (err error)
	AddRecord(ctx context.Context, logId []byte, clog *consensusproto.Record) (err error)
	WatchLog(ctx context.Context, logId []byte) (stream consensusproto.DRPCConsensus_WatchLogClient, err error)
	app.Component
}

type service struct {
	pool     pool.Pool
	nodeconf nodeconf.Service
	client   consensusproto.DRPCConsensusClient
	mu       sync.Mutex
}

func (s *service) Init(a *app.App) (err error) {
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.nodeconf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) getClient(ctx context.Context) (consensusproto.DRPCConsensusClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	peers := s.nodeconf.ConsensusPeers()
	rand.Shuffle(len(peers), func(i, j int) {
		peers[i], peers[j] = peers[j], peers[i]
	})

	for _, peerId := range peers {
		peer, err := s.pool.Get(ctx, peerId)
		if err != nil {
			log.Warn("can't connect to peer", zap.String("peerId", peerId), zap.Error(err))
			continue
		} else {
			s.client = consensusproto.NewDRPCConsensusClient(peer)
			return s.client, nil
		}
	}
	return nil, fmt.Errorf("unable to connect any consensus node")
}

func (s *service) AddLog(ctx context.Context, clog *consensusproto.Log) (err error) {
	cl, err := s.getClient(ctx)
	if err != nil {
		return
	}
	_, err = cl.AddLog(ctx, &consensusproto.AddLogRequest{
		Log: clog,
	})
	return
}

func (s *service) AddRecord(ctx context.Context, logId []byte, clog *consensusproto.Record) (err error) {
	cl, err := s.getClient(ctx)
	if err != nil {
		return
	}
	_, err = cl.AddRecord(ctx, &consensusproto.AddRecordRequest{
		LogId:  logId,
		Record: clog,
	})
	return
}

func (s *service) WatchLog(ctx context.Context, logId []byte) (stream consensusproto.DRPCConsensus_WatchLogClient, err error) {
	cl, err := s.getClient(ctx)
	if err != nil {
		return
	}
	return cl.WatchLog(ctx, &consensusproto.WatchLogRequest{
		LogId: logId,
	})
}
