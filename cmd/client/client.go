package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/transport"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/sec"
	"go.uber.org/zap"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"syscall"
	"time"
)

var log = logger.NewNamed("client")

var (
	flagConfigFile = flag.String("c", "etc/config.yml", "path to config file")
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
	a.Register(conf).
		Register(transport.New()).
		Register(&Client{})

	// start app
	if err := a.Start(ctx); err != nil {
		log.Fatal("can't start app", zap.Error(err))
	}
	log.Info("app started", zap.String("version", a.Version()))

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

type Client struct {
	conf config.GrpcServer
	tr   transport.Service
	sc   sec.SecureConn
}

func (c *Client) Init(ctx context.Context, a *app.App) (err error) {
	c.tr = a.MustComponent(transport.CName).(transport.Service)
	c.conf = a.MustComponent(config.CName).(*config.Config).GrpcServer
	return nil
}

func (c *Client) Name() (name string) {
	return "testClient"
}

func (c *Client) Run(ctx context.Context) (err error) {
	tcpConn, err := net.Dial("tcp", c.conf.ListenAddrs[0])
	if err != nil {
		return
	}
	c.sc, err = c.tr.TLSConn(ctx, tcpConn)
	if err != nil {
		return
	}
	log.Info("connected with server", zap.String("serverPeer", c.sc.RemotePeer().String()), zap.String("per", c.sc.LocalPeer().String()))

	dconn := drpcconn.New(c.sc)
	stream, err := dconn.NewStream(ctx, "", enc{})
	if err != nil {
		return
	}
	go c.handleStream(stream)
	return nil
}

func (c *Client) handleStream(stream drpc.Stream) {
	var err error
	defer func() {
		log.Info("stream closed", zap.Error(err))
	}()
	var n int64 = 100000
	for i := int64(0); i < n; i++ {
		st := time.Now()
		if err = stream.MsgSend(&syncproto.SyncMessage{Seq: i}, enc{}); err != nil {
			if err == io.EOF {
				return
			}
			log.Fatal("send error", zap.Error(err))
		}
		log.Debug("message sent", zap.Int64("seq", i))
		msg := &syncproto.SyncMessage{}
		if err := stream.MsgRecv(msg, enc{}); err != nil {
			if err == io.EOF {
				return
			}
			log.Error("msg recv error", zap.Error(err))
		}
		log.Debug("message received", zap.Int64("seq", msg.Seq), zap.Duration("dur", time.Since(st)))
		time.Sleep(time.Second)
	}
}

func (c *Client) Close(ctx context.Context) (err error) {
	if c.sc != nil {
		return c.sc.Close()
	}
	return
}

type enc struct{}

func (e enc) Marshal(msg drpc.Message) ([]byte, error) {
	return msg.(proto.Marshaler).Marshal()
}

func (e enc) Unmarshal(buf []byte, msg drpc.Message) error {
	return msg.(proto.Unmarshaler).Unmarshal(buf)
}
