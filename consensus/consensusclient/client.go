package consensusclient

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
)

const CName = "consensus.client"

var log = logger.NewNamed(CName)

func New() Service {
	return new(service)
}

type Service interface {
	AddLog(ctx context.Context, clog *consensusproto.Log) (err error)
	AddRecord(ctx context.Context, logId []byte, clog *consensusproto.Record) (err error)
	WatchLog(ctx context.Context) (stream Stream, err error)
	app.Component
}

type service struct {
	pool     pool.Pool
	nodeconf nodeconf.Service
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
	peer, err := s.pool.GetOneOf(ctx, s.nodeconf.ConsensusPeers())
	if err != nil {
		return nil, err
	}
	return consensusproto.NewDRPCConsensusClient(peer), nil
}

func (s *service) dialClient(ctx context.Context) (consensusproto.DRPCConsensusClient, error) {
	peer, err := s.pool.DialOneOf(ctx, s.nodeconf.ConsensusPeers())
	if err != nil {
		return nil, err
	}
	return consensusproto.NewDRPCConsensusClient(peer), nil
}

func (s *service) AddLog(ctx context.Context, clog *consensusproto.Log) (err error) {
	cl, err := s.getClient(ctx)
	if err != nil {
		return
	}
	if _, err = cl.AddLog(ctx, &consensusproto.AddLogRequest{
		Log: clog,
	}); err != nil {
		return rpcerr.Unwrap(err)
	}
	return
}

func (s *service) AddRecord(ctx context.Context, logId []byte, clog *consensusproto.Record) (err error) {
	cl, err := s.getClient(ctx)
	if err != nil {
		return
	}
	if _, err = cl.AddRecord(ctx, &consensusproto.AddRecordRequest{
		LogId:  logId,
		Record: clog,
	}); err != nil {
		return rpcerr.Unwrap(err)
	}
	return
}

func (s *service) WatchLog(ctx context.Context) (st Stream, err error) {
	cl, err := s.dialClient(ctx)
	if err != nil {
		return
	}
	rpcStream, err := cl.WatchLog(ctx)
	if err != nil {
		return nil, rpcerr.Unwrap(err)
	}
	return runStream(rpcStream), nil
}
