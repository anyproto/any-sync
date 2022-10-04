package stream

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/db"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ocache"
	"github.com/mr-tron/base58"
	"go.uber.org/zap"
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

type Stream interface {
	LogId() []byte
	Records() []consensus.Record
	WaitRecords() []consensus.Record
	Close()
}

type Service interface {
	Subscribe(ctx context.Context, logId []byte) (stream Stream, err error)
	app.ComponentRunnable
}

type service struct {
	db    db.Service
	cache ocache.OCache
}

func (s *service) Init(a *app.App) (err error) {
	s.db = a.MustComponent(db.CName).(db.Service)
	s.cache = ocache.New(s.loadLog,
		ocache.WithTTL(cacheTTL),
		ocache.WithRefCounter(false),
		ocache.WithLogger(log.Named("cache").Sugar()),
	)

	return s.db.SetChangeReceiver(s.receiveChange)
}

func (s *service) Subscribe(ctx context.Context, logId []byte) (Stream, error) {
	obj, err := s.getObject(ctx, logId)
	if err != nil {
		return nil, err
	}
	return obj.NewStream(), nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	return nil
}

func (s *service) loadLog(ctx context.Context, id string) (value ocache.Object, err error) {
	if ctxLog := ctx.Value(ctxLogKey); ctxLog != nil {
		return &object{
			logId:   ctxLog.(consensus.Log).Id,
			records: ctxLog.(consensus.Log).Records,
			streams: make(map[uint32]*stream),
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
		streams: make(map[uint32]*stream),
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
