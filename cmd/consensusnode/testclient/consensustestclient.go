package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/dialer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/secure"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusclient"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto/consensuserrs"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/account"
	"go.uber.org/zap"
	"gopkg.in/mgo.v2/bson"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var log = logger.NewNamed("main")

var (
	flagConfigFile = flag.String("c", "etc/consensus-config.yml", "path to config file")
	flagVersion    = flag.Bool("v", false, "show version and exit")
	flagHelp       = flag.Bool("h", false, "show help and exit")
)

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Println(app.VersionDescription())
		return
	}
	if *flagHelp {
		flag.PrintDefaults()
		return
	}

	if debug, ok := os.LookupEnv("ANYPROF"); ok && debug != "" {
		go func() {
			http.ListenAndServe(debug, nil)
		}()
	}

	// create app
	ctx := context.Background()
	a := new(app.App)

	// open config file
	conf, err := config.NewFromFile(*flagConfigFile)
	if err != nil {
		log.Fatal("can't open config file", zap.Error(err))
	}

	// bootstrap components
	a.Register(conf)
	Bootstrap(a)

	// start app
	if err := a.Start(ctx); err != nil {
		log.Fatal("can't start app", zap.Error(err))
	}
	log.Info("app started", zap.String("version", a.Version()))

	if err := testClient(a.MustComponent(consensusclient.CName).(consensusclient.Service)); err != nil {
		log.Fatal("test error", zap.Error(err))
	} else {
		log.Info("test success!")
	}

	// wait exit signal
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-exit
	log.Info("received exit signal, stop app...", zap.String("signal", fmt.Sprint(sig)))

	// close app
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := a.Close(ctx); err != nil {
		log.Fatal("close error", zap.Error(err))
	} else {
		log.Info("goodbye!")
	}
	time.Sleep(time.Second / 3)
}

func Bootstrap(a *app.App) {
	a.Register(account.New()).
		Register(secure.New()).
		Register(nodeconf.New()).
		Register(dialer.New()).
		Register(pool.New()).
		Register(consensusclient.New())
}

func testClient(service consensusclient.Service) (err error) {
	if err = testCreateLogAndRecord(service); err != nil {
		return err
	}
	if err = testStream(service); err != nil {
		return err
	}
	return
}

func testCreateLogAndRecord(service consensusclient.Service) (err error) {
	ctx := context.Background()

	// create log
	newLogId := []byte(bson.NewObjectId())
	st := time.Now()
	lastRecId := []byte(bson.NewObjectId())
	err = service.AddLog(ctx, &consensusproto.Log{
		Id: newLogId,
		Records: []*consensusproto.Record{
			{
				Id:          lastRecId,
				Payload:     []byte("test"),
				CreatedUnix: uint64(time.Now().Unix()),
			},
		},
	})
	if err != nil {
		return err
	}
	log.Info("log created", zap.String("id", bson.ObjectId(newLogId).Hex()), zap.Duration("dur", time.Since(st)))

	// create log with same id
	st = time.Now()

	err = service.AddLog(ctx, &consensusproto.Log{
		Id: newLogId,
		Records: []*consensusproto.Record{
			{
				Id:          lastRecId,
				Payload:     []byte("test"),
				CreatedUnix: uint64(time.Now().Unix()),
			},
		},
	})
	if err != consensuserrs.ErrLogExists {
		return fmt.Errorf("unexpected error: '%v' want LogExists", zap.Error(err))
	}
	err = nil
	log.Info("log duplicate checked", zap.Duration("dur", time.Since(st)))

	// create record
	st = time.Now()
	recId := []byte(bson.NewObjectId())
	err = service.AddRecord(ctx, newLogId, &consensusproto.Record{
		Id:          []byte(bson.NewObjectId()),
		PrevId:      lastRecId,
		CreatedUnix: uint64(time.Now().Unix()),
	})
	if err != nil {
		return err
	}
	lastRecId = recId
	log.Info("record created", zap.String("id", bson.ObjectId(lastRecId).Hex()), zap.Duration("dur", time.Since(st)))

	// record conflict
	st = time.Now()
	err = service.AddRecord(ctx, newLogId, &consensusproto.Record{
		Id:          []byte(bson.NewObjectId()),
		PrevId:      []byte(bson.NewObjectId()),
		CreatedUnix: uint64(time.Now().Unix()),
	})
	if err != consensuserrs.ErrConflict {
		return fmt.Errorf("unexpected error: '%v' want Conflict", zap.Error(err))
	}
	err = nil
	log.Info("conflict record checked", zap.Duration("dur", time.Since(st)))

	return
}

func testStream(service consensusclient.Service) (err error) {
	ctx := context.Background()

	// create log
	newLogId := []byte(bson.NewObjectId())
	st := time.Now()
	lastRecId := []byte(bson.NewObjectId())
	err = service.AddLog(ctx, &consensusproto.Log{
		Id: newLogId,
		Records: []*consensusproto.Record{
			{
				Id:          lastRecId,
				Payload:     []byte("test"),
				CreatedUnix: uint64(time.Now().Unix()),
			},
		},
	})
	if err != nil {
		return err
	}
	log.Info("log created", zap.String("id", bson.ObjectId(newLogId).Hex()), zap.Duration("dur", time.Since(st)))

	stream, err := service.WatchLog(ctx, newLogId)
	if err != nil {
		return err
	}
	defer stream.Close()
	sr := readStream(stream)
	for i := 0; i < 10; i++ {
		st = time.Now()
		recId := []byte(bson.NewObjectId())
		err = service.AddRecord(ctx, newLogId, &consensusproto.Record{
			Id:          recId,
			PrevId:      lastRecId,
			CreatedUnix: uint64(time.Now().Unix()),
		})
		if err != nil {
			return err
		}
		lastRecId = recId
		log.Info("record created", zap.String("id", bson.ObjectId(lastRecId).Hex()), zap.Duration("dur", time.Since(st)))
	}
	fmt.Println(sr.log.Records)
	return nil
}

func readStream(stream consensusproto.DRPCConsensus_WatchLogClient) *streamReader {
	sr := &streamReader{stream: stream}
	go sr.read()
	return sr
}

type streamReader struct {
	stream consensusproto.DRPCConsensus_WatchLogClient
	log    *consensusproto.Log
}

func (sr *streamReader) read() {
	for {
		event, err := sr.stream.Recv()
		if err != nil {
			return
		}
		fmt.Println("received event", event)
		if sr.log == nil {
			sr.log = &consensusproto.Log{
				Id:      event.LogId,
				Records: event.Records,
			}
		} else {
			sr.log.Records = append(event.Records, sr.log.Records...)
		}
	}
}
