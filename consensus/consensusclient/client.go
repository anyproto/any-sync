//go:generate mockgen -destination mock_consensusclient/mock_consensusclient.go github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusclient Service
package consensusclient

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/rpc/rpcerr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	_ "github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto/consensuserr"
	"go.uber.org/zap"
	"sync"
	"time"
)

const CName = "consensus.client"

var log = logger.NewNamed(CName)

var (
	ErrWatcherExists    = errors.New("watcher exists")
	ErrWatcherNotExists = errors.New("watcher not exists")
)

func New() Service {
	return new(service)
}

// Watcher watches new events by specified logId
type Watcher interface {
	AddConsensusRecords(recs []*consensusproto.Record)
	AddConsensusError(err error)
}

type Service interface {
	// AddLog adds new log to consensus servers
	AddLog(ctx context.Context, clog *consensusproto.Log) (err error)
	// AddRecord adds new record to consensus servers
	AddRecord(ctx context.Context, logId []byte, clog *consensusproto.Record) (err error)
	// Watch starts watching to given logId and calls watcher when any relative event received
	Watch(logId []byte, w Watcher) (err error)
	// UnWatch stops watching given logId and removes watcher
	UnWatch(logId []byte) (err error)
	app.ComponentRunnable
}

type service struct {
	pool     pool.Pool
	nodeconf nodeconf.Service

	watchers map[string]Watcher
	stream   *stream
	close    chan struct{}
	mu       sync.Mutex
}

func (s *service) Init(a *app.App) (err error) {
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.nodeconf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.watchers = make(map[string]Watcher)
	s.close = make(chan struct{})
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(_ context.Context) error {
	go s.streamWatcher()
	return nil
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

func (s *service) Watch(logId []byte, w Watcher) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.watchers[string(logId)]; ok {
		return ErrWatcherExists
	}
	s.watchers[string(logId)] = w
	if s.stream != nil {
		if wErr := s.stream.WatchIds([][]byte{logId}); wErr != nil {
			log.Warn("WatchIds error", zap.Error(wErr))
		}
	}
	return
}

func (s *service) UnWatch(logId []byte) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.watchers[string(logId)]; !ok {
		return ErrWatcherNotExists
	}
	delete(s.watchers, string(logId))
	if s.stream != nil {
		if wErr := s.stream.UnwatchIds([][]byte{logId}); wErr != nil {
			log.Warn("UnWatchIds error", zap.Error(wErr))
		}
	}
	return
}

func (s *service) openStream(ctx context.Context) (st *stream, err error) {
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

func (s *service) streamWatcher() {
	var (
		err error
		st  *stream
		i   int
	)
	for {
		// open stream
		if st, err = s.openStream(context.Background()); err != nil {
			// can't open stream, we will retry until success connection or close
			if i < 60 {
				i++
			}
			sleepTime := time.Second * time.Duration(i)
			log.Error("watch log error", zap.Error(err), zap.Duration("waitTime", sleepTime))
			select {
			case <-time.After(sleepTime):
				continue
			case <-s.close:
				return
			}
		}
		i = 0

		// collect ids and setup stream
		s.mu.Lock()
		var logIds = make([][]byte, 0, len(s.watchers))
		for id := range s.watchers {
			logIds = append(logIds, []byte(id))
		}
		s.stream = st
		s.mu.Unlock()

		// restore subscriptions
		if len(logIds) > 0 {
			if err = s.stream.WatchIds(logIds); err != nil {
				log.Error("watch ids error", zap.Error(err))
				continue
			}
		}

		// read stream
		if err = s.streamReader(); err != nil {
			log.Error("stream read error", zap.Error(err))
			continue
		}
		return
	}
}

func (s *service) streamReader() error {
	for {
		events := s.stream.WaitLogs()
		if len(events) == 0 {
			return s.stream.Err()
		}
		for _, e := range events {
			if w, ok := s.watchers[string(e.LogId)]; ok {
				if e.Error == nil {
					w.AddConsensusRecords(e.Records)
				} else {
					w.AddConsensusError(rpcerr.Err(uint64(e.Error.Error)))
				}
			} else {
				log.Warn("received unexpected log id", zap.Binary("logId", e.LogId))
			}
		}
	}
}

func (s *service) Close(_ context.Context) error {
	s.mu.Lock()
	if s.stream != nil {
		_ = s.stream.Close()
	}
	s.mu.Unlock()
	select {
	case <-s.close:
	default:
		close(s.close)
	}
	return nil
}
