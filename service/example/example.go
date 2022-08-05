package example

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"go.uber.org/zap"
	"strings"
	"time"
)

var log = logger.NewNamed("example")

type Example struct {
	pool     pool.Pool
	peerConf config.PeerList
}

func (e *Example) Init(ctx context.Context, a *app.App) (err error) {
	e.pool = a.MustComponent(pool.CName).(pool.Pool)
	e.peerConf = a.MustComponent(config.CName).(*config.Config).PeerList

	// subscribe for sync messages
	e.pool.AddHandler(syncproto.MessageType_MessageTypeSync, e.syncHandler)
	return
}

func (e *Example) Name() (name string) {
	return "example"
}

func (e *Example) Run(ctx context.Context) (err error) {
	// dial manually with all peers
	for _, rp := range e.peerConf.Remote {
		if er := e.pool.DialAndAddPeer(ctx, rp.PeerId); er != nil {
			log.Info("can't dial to peer", zap.Error(er))
		} else {
			log.Info("connected with peer", zap.String("peerId", rp.PeerId))
		}
	}
	go e.doRequests()
	return nil
}

func (e *Example) syncHandler(ctx context.Context, msg *pool.Message) (err error) {
	data := string(msg.Data) // you need unmarshal this bytes
	log.Info("msg received", zap.String("peerId", msg.Peer().Id()), zap.String("data", data))

	if strings.HasPrefix(data, "ack:") {
		if err = msg.Ack(); err != nil {
			log.Error("ack error", zap.Error(err))
		}
	} else if strings.HasPrefix(data, "ackErr:") {
		if err = msg.AckError(42, "ack error description"); err != nil {
			log.Error("ackErr error", zap.Error(err))
		}
	} else if strings.HasPrefix(data, "reply:") {
		if err = msg.Reply([]byte("reply for:" + strings.TrimPrefix(data, "reply:"))); err != nil {
			log.Error("reply error", zap.Error(err))
		}
	}
	return nil
}

func (e *Example) doRequests() {
	time.Sleep(time.Second)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	st := time.Now()
	err := e.pool.SendAndWait(ctx, e.peerConf.Remote[0].PeerId, &syncproto.Message{
		Header: &syncproto.Header{Type: syncproto.MessageType_MessageTypeSync},
		Data:   []byte("ack: something"),
	})
	log.Info("sent with ack:", zap.Error(err), zap.Duration("dur", time.Since(st)))
}

func (e *Example) Close(ctx context.Context) (err error) {
	return
}
