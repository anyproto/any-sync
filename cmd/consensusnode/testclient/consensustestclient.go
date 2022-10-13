package main

import (
	"bytes"
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
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto/consensuserr"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/node/account"
	"go.uber.org/zap"
	"gopkg.in/mgo.v2/bson"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
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
	rand.Seed(time.Now().UnixNano())
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

	b := &bench{service: a.MustComponent(consensusclient.CName).(consensusclient.Service)}
	b.run()

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
	if err != consensuserr.ErrLogExists {
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
	if err != consensuserr.ErrConflict {
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

	stream, err := service.WatchLog(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	st = time.Now()
	if err = stream.WatchIds([][]byte{newLogId}); err != nil {
		return err
	}
	log.Info("watch", zap.String("id", bson.ObjectId(newLogId).Hex()), zap.Duration("dur", time.Since(st)))

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
	sr.validate()
	return nil
}

func readStream(stream consensusclient.Stream) *streamReader {
	sr := &streamReader{stream: stream, logs: map[string]*consensusproto.Log{}}
	go sr.read()
	return sr
}

type streamReader struct {
	stream consensusclient.Stream
	logs   map[string]*consensusproto.Log
}

func (sr *streamReader) read() {
	for {
		recs := sr.stream.WaitLogs()
		if len(recs) == 0 {
			return
		}
		for _, rec := range recs {
			if el, ok := sr.logs[string(rec.Id)]; !ok {
				sr.logs[string(rec.Id)] = &consensusproto.Log{
					Id:      rec.Id,
					Records: rec.Records,
				}
			} else {
				el.Records = append(rec.Records, el.Records...)
				sr.logs[string(rec.Id)] = el
			}
		}
	}
}

func (sr *streamReader) validate() {
	var lc, rc int
	for _, log := range sr.logs {
		lc++
		rc += len(log.Records)
		validateLog(log)
	}
	fmt.Println("logs valid; log count:", lc, "records:", rc)
}

func validateLog(log *consensusproto.Log) {
	var prevId []byte
	for _, rec := range log.Records {
		if len(prevId) != 0 {
			if !bytes.Equal(prevId, rec.Id) {
				panic(fmt.Sprintf("invalid log: %+v", log))
			}
		}
		prevId = rec.PrevId
	}
}

type bench struct {
	service consensusclient.Service
	stream  consensusclient.Stream
}

func (b *bench) run() {
	var err error
	b.stream, err = b.service.WatchLog(context.Background())
	if err != nil {
		panic(err)
	}
	defer b.stream.Close()
	sr := readStream(b.stream)
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		time.Sleep(time.Second / 100)
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.client()
		}()
		fmt.Println("total streams:", i+1)
	}
	wg.Wait()
	sr.validate()
}

func (b *bench) client() {
	ctx := context.Background()

	// create log
	newLogId := []byte(bson.NewObjectId())
	lastRecId := []byte(bson.NewObjectId())
	err := b.service.AddLog(ctx, &consensusproto.Log{
		Id: newLogId,
		Records: []*consensusproto.Record{
			{
				Id:          lastRecId,
				Payload:     []byte("test"),
				CreatedUnix: uint64(time.Now().Unix()),
			},
		},
	})
	for i := 0; i < 5; i++ {
		fmt.Println("watch", bson.ObjectId(newLogId).Hex())
		if err = b.stream.WatchIds([][]byte{newLogId}); err != nil {
			panic(err)
		}
		for i := 0; i < rand.Intn(20); i++ {
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(10000)))
			recId := []byte(bson.NewObjectId())
			err = b.service.AddRecord(ctx, newLogId, &consensusproto.Record{
				Id:          recId,
				PrevId:      lastRecId,
				Payload:     []byte("some payload 1 2 3 4 5  6 6 7     oijoj"),
				CreatedUnix: uint64(time.Now().Unix()),
			})
			if err != nil {
				panic(err)
			}
			lastRecId = recId
		}
		if err = b.stream.UnwatchIds([][]byte{newLogId}); err != nil {
			panic(err)
		}
		fmt.Println("unwatch", bson.ObjectId(newLogId).Hex())
		time.Sleep(time.Minute * 1)
	}
}
