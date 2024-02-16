//go:generate mockgen -destination mock_consensusclient/mock_consensusclient.go github.com/anyproto/any-sync/consensus/consensusclient Service
package consensusclient

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"
)

const CName = "consensus.consensusclient"

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
	AddConsensusRecords(recs []*consensusproto.RawRecordWithId)
	AddConsensusError(err error)
}

type Service interface {
	// AddLog adds new log to consensus servers
	AddLog(ctx context.Context, logId string, rec *consensusproto.RawRecordWithId) (err error)
	// DeleteLog deletes the log from the consensus node
	DeleteLog(ctx context.Context, logId string) (err error)
	// AddRecord adds new record to consensus servers
	AddRecord(ctx context.Context, logId string, rec *consensusproto.RawRecord) (record *consensusproto.RawRecordWithId, err error)
	// Watch starts watching to given logId and calls watcher when any relative event received
	Watch(logId string, w Watcher) (err error)
	// UnWatch stops watching given logId and removes watcher
	UnWatch(logId string) (err error)
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

func (s *service) doClient(ctx context.Context, fn func(cl consensusproto.DRPCConsensusClient) error) error {
	peer, err := s.pool.GetOneOf(ctx, s.nodeconf.ConsensusPeers())
	if err != nil {
		return err
	}
	dc, err := peer.AcquireDrpcConn(ctx)
	if err != nil {
		return err
	}
	defer peer.ReleaseDrpcConn(dc)
	return fn(consensusproto.NewDRPCConsensusClient(dc))
}

func (s *service) AddLog(ctx context.Context, logId string, rec *consensusproto.RawRecordWithId) (err error) {
	return s.doClient(ctx, func(cl consensusproto.DRPCConsensusClient) error {
		if _, err = cl.LogAdd(ctx, &consensusproto.LogAddRequest{
			LogId:  logId,
			Record: rec,
		}); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
}

func (s *service) DeleteLog(ctx context.Context, logId string) (err error) {
	return s.doClient(ctx, func(cl consensusproto.DRPCConsensusClient) error {
		if _, err = cl.LogDelete(ctx, &consensusproto.LogDeleteRequest{
			LogId: logId,
		}); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
}

func (s *service) AddRecord(ctx context.Context, logId string, rec *consensusproto.RawRecord) (record *consensusproto.RawRecordWithId, err error) {
	err = s.doClient(ctx, func(cl consensusproto.DRPCConsensusClient) error {
		if record, err = cl.RecordAdd(ctx, &consensusproto.RecordAddRequest{
			LogId:  logId,
			Record: rec,
		}); err != nil {
			return rpcerr.Unwrap(err)
		}
		return nil
	})
	return
}

func (s *service) Watch(logId string, w Watcher) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.watchers[logId]; ok {
		return ErrWatcherExists
	}
	s.watchers[logId] = w
	if s.stream != nil {
		if wErr := s.stream.WatchIds([]string{logId}); wErr != nil {
			log.Warn("WatchIds error", zap.Error(wErr))
		}
	}
	return
}

func (s *service) UnWatch(logId string) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.watchers[logId]; !ok {
		return ErrWatcherNotExists
	}
	delete(s.watchers, logId)
	if s.stream != nil {
		if wErr := s.stream.UnwatchIds([]string{logId}); wErr != nil {
			log.Warn("UnWatchIds error", zap.Error(wErr))
		}
	}
	return
}

func (s *service) openStream(ctx context.Context) (st *stream, err error) {
	pr, err := s.pool.GetOneOf(ctx, s.nodeconf.ConsensusPeers())
	if err != nil {
		return nil, err
	}
	pr.SetTTL(time.Hour * 24)
	dc, err := pr.AcquireDrpcConn(ctx)
	if err != nil {
		return nil, err
	}
	rpcStream, err := consensusproto.NewDRPCConsensusClient(dc).LogWatch(ctx)
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
		var logIds = make([]string, 0, len(s.watchers))
		for id := range s.watchers {
			logIds = append(logIds, id)
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
		s.mu.Lock()
		for _, e := range events {
			if w, ok := s.watchers[e.LogId]; ok {
				if e.Error == nil {
					w.AddConsensusRecords(e.Records)
				} else {
					w.AddConsensusError(rpcerr.Err(uint64(e.Error.Error)))
				}
			} else {
				log.Warn("received unexpected log id", zap.String("logId", e.LogId))
			}
		}
		s.mu.Unlock()
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
