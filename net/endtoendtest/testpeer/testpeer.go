package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/endtoendtest/testpeer/testproto"
	"github.com/anyproto/any-sync/net/peerservice"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport/quic"
	"github.com/anyproto/any-sync/net/transport/yamux"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
)

var (
	fYamuxAddr = flag.String("yamux", "127.0.0.1:60642", "server yamux listen addr")
	fQuicAddr  = flag.String("quic", "127.0.0.1:60643", "server quic listen addr")
	fPprofAddr = flag.String("pprof", "127.0.0.1:6061", "pprof addr")
	fSnappy    = flag.Bool("snappy", false, "use snappy")

	fPeerAddr = flag.String("peer", "quic://127.0.0.1:60643", "peer addr")
	fPeerId   = flag.String("peerId", "", "peer id")

	fUnary        = flag.String("unary", "10-10000", "unary")
	fClientStream = flag.String("cstream", "10-10000", "client stream")
)

type config struct {
	Yamux    yamux.Config
	Quic     quic.Config
	Snappy   bool
	PeerId   string
	PeerAddr string
	NodeConf nodeconf.Configuration
}

func (c *config) Name() string                { return "config" }
func (c *config) Init(_ *app.App) (err error) { return }

func (c *config) GetYamux() yamux.Config              { return c.Yamux }
func (c *config) GetQuic() quic.Config                { return c.Quic }
func (c *config) GetNodeConf() nodeconf.Configuration { return c.NodeConf }
func (c *config) GetDrpc() rpc.Config                 { return rpc.Config{Snappy: *fSnappy} }

type nodeConfSource struct {
}

func (n *nodeConfSource) Init(a *app.App) (err error) {
	return nil
}

func (n *nodeConfSource) Name() (name string) {
	return nodeconf.CNameSource
}

func (n *nodeConfSource) GetLast(ctx context.Context, currentId string) (c nodeconf.Configuration, err error) {
	return c, nodeconf.ErrConfigurationNotFound
}

func (n *nodeConfSource) IsNetworkNeedsUpdate(ctx context.Context) (bool, error) {
	return false, nil
}

type nodeConfStore struct {
}

func (n *nodeConfStore) Init(a *app.App) (err error) {
	return nil
}

func (n *nodeConfStore) Name() (name string) {
	return nodeconf.CNameStore
}

func (n *nodeConfStore) GetLast(ctx context.Context, netId string) (c nodeconf.Configuration, err error) {
	return c, nodeconf.ErrConfigurationNotFound
}

func (n *nodeConfStore) SaveLast(ctx context.Context, c nodeconf.Configuration) (err error) {
	return nil
}

var ctx = context.Background()

func main() {
	flag.Parse()

	lc := logger.Config{
		Production:   false,
		DefaultLevel: "info",
	}
	lc.ApplyGlobal()

	if *fPprofAddr != "" {
		go func() {
			log.Println(http.ListenAndServe(*fPprofAddr, nil))
		}()
	}

	accountService := &accounttest.AccountTestService{}
	client := &testClient{}

	a := new(app.App)
	conf := makeConfig()

	a.Register(conf).
		Register(accountService).
		Register(pool.New()).
		Register(peerservice.New()).
		Register(nodeconf.New()).
		Register(secureservice.New()).
		Register(&nodeConfSource{}).
		Register(&nodeConfStore{}).
		Register(server.New()).
		Register(&testServer{}).
		Register(client).
		Register(quic.New()).
		Register(yamux.New())

	if err := a.Start(ctx); err != nil {
		panic(err)
	}
	fmt.Println(accountService.Account().PeerId)

	var isClient = *fPeerId != "" && *fPeerAddr != ""

	if isClient {
		a.MustComponent(peerservice.CName).(peerservice.PeerService).SetPeerAddrs(*fPeerId, []string{*fPeerAddr})
		if *fUnary != "" {
			unaryParams := strings.Split(*fUnary, "-")
			if len(unaryParams) == 2 {
				n, _ := strconv.Atoi(unaryParams[1])
				proc, _ := strconv.Atoi(unaryParams[0])
				if err := client.UnaryN(ctx, n, proc); err != nil {
					fmt.Println("unary:", err)
				}
			}
		}
		if *fClientStream != "" {
			params := strings.Split(*fUnary, "-")
			if len(params) == 2 {
				n, _ := strconv.Atoi(params[1])
				proc, _ := strconv.Atoi(params[0])
				if err := client.ClientStream(ctx, n, proc); err != nil {
					fmt.Println("client stream:", err)
				}
			}
		}
	} else {
		var sig = make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-sig:
			fmt.Println("catch", sig)
		}
	}

	if err := a.Close(ctx); err != nil {
		panic(err)
	}
}

func makeConfig() *config {

	var yamuxAddrs []string
	if *fPeerId == "" && *fYamuxAddr != "" {
		yamuxAddrs = append(yamuxAddrs, *fYamuxAddr)
	}
	var quicAddrs []string
	if *fPeerId == "" && *fQuicAddr != "" {
		quicAddrs = append(quicAddrs, *fQuicAddr)
	}

	return &config{
		Yamux: yamux.Config{
			ListenAddrs:        yamuxAddrs,
			WriteTimeoutSec:    5,
			DialTimeoutSec:     5,
			KeepAlivePeriodSec: 30,
		},
		Quic: quic.Config{
			ListenAddrs:        quicAddrs,
			WriteTimeoutSec:    5,
			DialTimeoutSec:     5,
			MaxStreams:         100,
			KeepAlivePeriodSec: 30,
		},
		NodeConf: nodeconf.Configuration{},
		PeerId:   *fPeerId,
		PeerAddr: *fPeerAddr,
	}
}

type testServer struct {
}

func (t *testServer) BiDirectionalStream(stream testproto.DRPCTest_BiDirectionalStreamStream) error {
	for {
		inMsg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err = stream.Send(inMsg); err != nil {
			return err
		}
	}
}

func (t *testServer) ServerStream(message *testproto.StreamMessage, stream testproto.DRPCTest_ServerStreamStream) error {
	for n := range message.Repeat {
		message.N = n
		if err := stream.Send(message); err != nil {
			return err
		}
	}
	return nil
}

func (t *testServer) ClientStream(stream testproto.DRPCTest_ClientStreamStream) error {
	var n int64
	for {
		inMsg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if inMsg.N != n {
			return fmt.Errorf("server: n mismatched expected %d got %d", n, inMsg.N)
		}
		n++
	}
	return stream.SendAndClose(&testproto.StreamMessage{
		N: n,
	})
}

func (t *testServer) Unary(ctx context.Context, message *testproto.StreamMessage) (*testproto.StreamMessage, error) {
	return &testproto.StreamMessage{
		ReqData: message.ReqData,
		Repeat:  message.Repeat,
		N:       message.N + 1,
	}, nil
}

func (t *testServer) Init(a *app.App) (err error) {
	srv := a.MustComponent(server.CName).(server.DRPCServer)
	return testproto.DRPCRegisterTest(srv, t)
}

func (t *testServer) Name() (name string) {
	return "testserver"
}

type testClient struct {
	pool   pool.Pool
	peerId string
}

func (c *testClient) Name() string {
	return "testclient"
}

func (c *testClient) Init(a *app.App) (err error) {
	c.pool = a.MustComponent(pool.CName).(pool.Pool)
	conf := a.MustComponent("config").(*config)
	c.peerId = conf.PeerId
	return nil
}

func (c *testClient) UnaryN(ctx context.Context, n, proc int) (err error) {
	st := time.Now()
	pr, err := c.pool.Get(ctx, c.peerId)
	if err != nil {
		return fmt.Errorf("get peer: %w", err)
	}
	fmt.Println("connected", time.Since(st))

	var data = strings.Repeat("x", 256)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(proc)

	for i := range n {
		g.Go(func() error {
			return pr.DoDrpc(ctx, func(conn drpc.Conn) error {
				cl := testproto.NewDRPCTestClient(conn)
				resp, err := cl.Unary(ctx, &testproto.StreamMessage{
					ReqData: data,
					Repeat:  int64(n),
					N:       int64(i),
				})
				if err != nil {
					return err
				}
				if resp.N != int64(i+1) {
					return fmt.Errorf("unary N mismatched: %d != %d", resp.N, n+1)
				}
				return nil
			})
		})
	}
	if err = g.Wait(); err != nil {
		return err
	}
	fmt.Printf("done %d req in %v; %.f2 per sec\n", n, time.Since(st), float64(n)/time.Since(st).Seconds())
	return nil
}

func (c *testClient) ClientStream(ctx context.Context, n, proc int) (err error) {
	st := time.Now()
	pr, err := c.pool.Get(ctx, c.peerId)
	if err != nil {
		return fmt.Errorf("get peer: %w", err)
	}
	fmt.Println("connected", time.Since(st))

	var data = strings.Repeat("x", 256)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(proc)
	msgsPerStream := int64(1000)
	for range n / int(msgsPerStream) {
		g.Go(func() error {
			return pr.DoDrpc(ctx, func(conn drpc.Conn) error {
				cl := testproto.NewDRPCTestClient(conn)
				stream, err := cl.ClientStream(ctx)
				if err != nil {
					return fmt.Errorf("client stream: %w", err)
				}
				for i := range msgsPerStream {
					if err = stream.Send(&testproto.StreamMessage{
						ReqData: data,
						Repeat:  msgsPerStream,
						N:       i,
					}); err != nil {
						return fmt.Errorf("stream send: %w", err)
					}
				}
				resp, err := stream.CloseAndRecv()
				if err != nil {
					return fmt.Errorf("close and recv: %w", err)
				}
				if resp.N != msgsPerStream {
					return fmt.Errorf("client stream N mismatched: %d != %d", resp.N, msgsPerStream)
				}
				return nil
			})
		})
	}
	if err = g.Wait(); err != nil {
		return err
	}
	fmt.Printf("done %d req in %v; %.f2 per sec\n", n, time.Since(st), float64(n)/time.Since(st).Seconds())
	return nil
}
