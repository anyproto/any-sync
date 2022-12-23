package stream

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/metric"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ocache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/db"
	"github.com/cheggaaa/mb/v3"
	"github.com/mr-tron/base58"
	"go.uber.org/zap"
	"sync/atomic"
	"time"
)

const CName = "consensus.stream"

var log = logger.NewNamed(CName)

var (
	cacheTTL = time.Minute
)

type ctxLog uint

const (
	ctxLogKey ctxLog = 1
)

func New() Service {
	return &service{}
}

// Service maintains a cache for logs (receive updates from db) and creates new stream objects with able to subscribe/unsubscribe to log ids
type Service interface {
	// NewStream creates new stream with able to watch and unwatch log ids
	NewStream() *Stream
	app.ComponentRunnable
}

type service struct {
	db           db.Service
	cache        ocache.OCache
	lastStreamId uint64
}

func (s *service) Init(a *app.App) (err error) {
	s.db = a.MustComponent(db.CName).(db.Service)
	cacheOpts := []ocache.Option{
		ocache.WithTTL(cacheTTL),
		ocache.WithLogger(log.Named("cache").Sugar()),
	}
	if ms := a.Component(metric.CName); ms != nil {
		cacheOpts = append(cacheOpts, ocache.WithPrometheus(ms.(metric.Metric).Registry(), "consensus", "logcache"))
	}
	s.cache = ocache.New(s.loadLog, cacheOpts...)

	return s.db.SetChangeReceiver(s.receiveChange)
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	return nil
}

func (s *service) NewStream() *Stream {
	return &Stream{
		id:     atomic.AddUint64(&s.lastStreamId, 1),
		logIds: make(map[string]struct{}),
		mb:     mb.New[consensus.Log](100),
		s:      s,
	}
}

// AddStream to object with given logId
func (s *service) AddStream(ctx context.Context, logId []byte, stream *Stream) (err error) {
	obj, err := s.getObject(ctx, logId)
	if err != nil {
		return err
	}
	obj.AddStream(stream)
	return
}

// RemoveStream from object with five logId
func (s *service) RemoveStream(ctx context.Context, logId []byte, streamId uint64) (err error) {
	obj, err := s.getObject(ctx, logId)
	if err != nil {
		return err
	}
	obj.RemoveStream(streamId)
	return
}

func (s *service) loadLog(ctx context.Context, id string) (value ocache.Object, err error) {
	if ctxLog := ctx.Value(ctxLogKey); ctxLog != nil {
		return &object{
			logId:   ctxLog.(consensus.Log).Id,
			records: ctxLog.(consensus.Log).Records,
			streams: make(map[uint64]*Stream),
		}, nil
	}
	logId := logIdFromString(id)
	dbLog, err := s.db.FetchLog(ctx, logId)
	if err != nil {
		return nil, err
	}
	return &object{
		logId:   dbLog.Id,
		records: dbLog.Records,
		streams: make(map[uint64]*Stream),
	}, nil
}

func (s *service) receiveChange(logId []byte, records []consensus.Record) {
	ctx := context.WithValue(context.Background(), ctxLogKey, consensus.Log{Id: logId, Records: records})
	obj, err := s.getObject(ctx, logId)
	if err != nil {
		log.Error("failed load object from cache", zap.Error(err))
		return
	}
	obj.AddRecords(records)
}

func (s *service) getObject(ctx context.Context, logId []byte) (*object, error) {
	id := logIdToString(logId)
	cacheObj, err := s.cache.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return cacheObj.(*object), nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return s.cache.Close()
}

func logIdToString(logId []byte) string {
	return base58.Encode(logId)
}

func logIdFromString(s string) []byte {
	logId, _ := base58.Decode(s)
	return logId
}
