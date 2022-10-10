package db

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto/consensuserr"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const CName = "consensus.db"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type ChangeReceiver func(logId []byte, records []consensus.Record)

type Service interface {
	// AddLog adds new log db
	AddLog(ctx context.Context, log consensus.Log) (err error)
	// AddRecord adds new record to existing log
	// returns consensuserr.ErrConflict if record didn't match or log not found
	AddRecord(ctx context.Context, logId []byte, record consensus.Record) (err error)
	// FetchLog gets log by id
	FetchLog(ctx context.Context, logId []byte) (log consensus.Log, err error)
	// SetChangeReceiver sets the receiver for updates, it must be called before app.Run stage
	SetChangeReceiver(receiver ChangeReceiver) (err error)
	app.ComponentRunnable
}

type service struct {
	conf           config.Mongo
	logColl        *mongo.Collection
	running        bool
	changeReceiver ChangeReceiver

	streamCtx    context.Context
	streamCancel context.CancelFunc
	listenerDone chan struct{}
}

func (s *service) Init(a *app.App) (err error) {
	s.conf = a.MustComponent(config.CName).(*config.Config).Mongo
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(s.conf.Connect))
	if err != nil {
		return err
	}
	s.logColl = client.Database(s.conf.Database).Collection(s.conf.LogCollection)
	s.running = true
	if s.changeReceiver != nil {
		if err = s.runStreamListener(ctx); err != nil {
			return err
		}
	}
	return
}

func (s *service) AddLog(ctx context.Context, l consensus.Log) (err error) {
	_, err = s.logColl.InsertOne(ctx, l)
	if mongo.IsDuplicateKeyError(err) {
		return consensuserr.ErrLogExists
	}
	return
}

type findLogQuery struct {
	Id []byte `bson:"_id"`
}

type findRecordQuery struct {
	Id           []byte `bson:"_id"`
	LastRecordId []byte `bson:"records.0.id"`
}

type updateOp struct {
	Push struct {
		Records struct {
			Each []consensus.Record `bson:"$each"`
			Pos  int                `bson:"$position"`
		} `bson:"records"`
	} `bson:"$push"`
}

func (s *service) AddRecord(ctx context.Context, logId []byte, record consensus.Record) (err error) {
	var upd updateOp
	upd.Push.Records.Each = []consensus.Record{record}
	result, err := s.logColl.UpdateOne(ctx, findRecordQuery{
		Id:           logId,
		LastRecordId: record.PrevId,
	}, upd)
	if err != nil {
		log.Error("addRecord update error", zap.Error(err))
		return consensuserr.ErrUnexpected
	}
	if result.ModifiedCount == 0 {
		return consensuserr.ErrConflict
	}
	return
}

func (s *service) FetchLog(ctx context.Context, logId []byte) (l consensus.Log, err error) {
	if err = s.logColl.FindOne(ctx, findLogQuery{Id: logId}).Decode(&l); err != nil {
		if err == mongo.ErrNoDocuments {
			err = consensuserr.ErrLogNotFound
		}
		return
	}
	return
}

func (s *service) SetChangeReceiver(receiver ChangeReceiver) (err error) {
	if s.running {
		return fmt.Errorf("set receiver must be called before Run")
	}
	s.changeReceiver = receiver
	return
}

type matchPipeline struct {
	Match struct {
		OT string `bson:"operationType"`
	} `bson:"$match"`
}

func (s *service) runStreamListener(ctx context.Context) (err error) {
	var mp matchPipeline
	mp.Match.OT = "update"
	stream, err := s.logColl.Watch(ctx, []matchPipeline{mp})
	if err != nil {
		return
	}
	s.listenerDone = make(chan struct{})
	s.streamCtx, s.streamCancel = context.WithCancel(context.Background())
	go s.streamListener(stream)
	return
}

type streamResult struct {
	DocumentKey struct {
		Id []byte `bson:"_id"`
	} `bson:"documentKey"`
	UpdateDescription struct {
		UpdateFields struct {
			Records []consensus.Record `bson:"records"`
		} `bson:"updatedFields"`
	} `bson:"updateDescription"`
}

func (s *service) streamListener(stream *mongo.ChangeStream) {
	defer close(s.listenerDone)
	for stream.Next(s.streamCtx) {
		var res streamResult
		if err := stream.Decode(&res); err != nil {
			// mongo driver maintains connections and handles reconnects so that the stream will work as usual in these cases
			// here we have an unexpected error and should stop any operations to avoid an inconsistent state between db and cache
			log.Fatal("stream decode error:", zap.Error(err))
		}
		s.changeReceiver(res.DocumentKey.Id, res.UpdateDescription.UpdateFields.Records)
	}
}

func (s *service) Close(ctx context.Context) (err error) {
	if s.logColl != nil {
		err = s.logColl.Database().Client().Disconnect(ctx)
		s.logColl = nil
	}
	if s.listenerDone != nil {
		s.streamCancel()
		<-s.listenerDone
	}
	return
}
