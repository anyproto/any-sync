package file

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/configuration"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool/handler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"math/rand"
	"sync/atomic"
	"time"
)

var log = logger.NewNamed("file")

const CName = "file"

type Service struct {
	pool          pool.Pool
	rndBuf        []byte
	conf          configuration.Service
	reqIn, reqOut uint64
	output, input uint64
}

func (s *Service) Init(ctx context.Context, a *app.App) (err error) {
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.conf = a.MustComponent(configuration.CName).(configuration.Service)
	rand.Seed(time.Now().UnixNano())
	s.rndBuf = make([]byte, 1024*1024*5)
	rand.Read(s.rndBuf)
	return
}

func (s *Service) Name() (name string) {
	return CName
}

func (s *Service) Run(ctx context.Context) (err error) {
	s.pool.AddHandler(syncproto.MessageType_MessageTypeFile, handler.Reply{ReplyHandler: s}.Handle)
	go s.bench()
	go s.benchLog()
	return nil
}

func (s *Service) bench() {
	ctx := context.Background()
	for {
		sleepTime := time.Second * time.Duration(rand.Intn(60)+10)
		log.Info("next bench in", zap.Duration("dur", sleepTime))
		time.Sleep(sleepTime)
		peer, err := s.conf.GetLast().OnePeer(ctx, fmt.Sprint(rand.Int()))
		if err != nil {
			log.Info("no peers")
			continue
		}
		peerId := peer.Id()
		for i := 0; i < 100000; i++ {
			resp, e := s.pool.SendAndWaitResponse(ctx, peerId, &syncproto.Message{
				Header: &syncproto.Header{
					Type: syncproto.MessageType_MessageTypeFile,
				},
			})
			if e != nil {
				log.Error("go error", zap.Error(e))
				break
			}
			atomic.AddUint64(&s.input, uint64(len(resp.Data)))
			atomic.AddUint64(&s.reqIn, 1)
		}
	}
}

func (s *Service) benchLog() {
	var prevRIn, prevROut, prevBIn, prevBOut uint64
	for {
		time.Sleep(time.Second)
		rIn := atomic.LoadUint64(&s.reqIn)
		rOut := atomic.LoadUint64(&s.reqOut)
		bIn := atomic.LoadUint64(&s.input)
		bOut := atomic.LoadUint64(&s.output)

		log.Info("speed",
			zap.Uint64("reqIn", rIn-prevRIn),
			zap.Uint64("in MB", (bIn-prevBIn)/1024/1024),
			zap.Uint64("reqOut", rOut-prevROut),
			zap.Uint64("out MB", (bOut-prevBOut)/1024/1024),
		)
		prevRIn = rIn
		prevROut = rOut
		prevBIn = bIn
		prevBOut = bOut
	}
}

func (s *Service) Handle(ctx context.Context, req []byte) (rep proto.Marshaler, err error) {
	size := rand.Intn(2*1024*1024) + 256
	atomic.AddUint64(&s.output, uint64(size))
	atomic.AddUint64(&s.reqOut, 1)
	return s.newRandMsg(size), nil
}

func (s *Service) newRandMsg(size int) *msg {
	return &msg{b: s.rndBuf[:size]}
}

func (s *Service) Close(ctx context.Context) (err error) {
	return nil
}

type msg struct {
	b []byte
}

func (m msg) Marshal() ([]byte, error) {
	return m.b, nil
}
