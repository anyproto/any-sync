package yamux

import (
	"context"
	"fmt"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/hashicorp/yamux"
	"go.uber.org/zap"
	"net"
	"time"
)

const CName = "net.transport.yamux"

var log = logger.NewNamed(CName)

func New() Yamux {
	return new(yamuxTransport)
}

// Yamux implements transport.Transport with tcp+yamux
type Yamux interface {
	transport.Transport
	app.ComponentRunnable
}

type yamuxTransport struct {
	secure   secureservice.SecureService
	accepter transport.Accepter
	conf     Config

	listeners     []net.Listener
	listCtx       context.Context
	listCtxCancel context.CancelFunc
	yamuxConf     *yamux.Config
}

func (y *yamuxTransport) Init(a *app.App) (err error) {
	y.secure = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	y.conf = a.MustComponent("config").(configGetter).GetYamux()
	y.yamuxConf = yamux.DefaultConfig()
	if y.conf.MaxStreams > 0 {
		y.yamuxConf.AcceptBacklog = y.conf.MaxStreams
	}
	y.yamuxConf.EnableKeepAlive = false
	y.yamuxConf.StreamOpenTimeout = time.Duration(y.conf.DialTimeoutSec) * time.Second
	y.yamuxConf.ConnectionWriteTimeout = time.Duration(y.conf.WriteTimeoutSec) * time.Second
	return
}

func (y *yamuxTransport) Name() string {
	return CName
}

func (y *yamuxTransport) Run(ctx context.Context) (err error) {
	if y.accepter == nil {
		return fmt.Errorf("can't run service without accepter")
	}
	for _, listAddr := range y.conf.ListenAddrs {
		list, err := net.Listen("tcp", listAddr)
		if err != nil {
			return err
		}
		y.listeners = append(y.listeners, list)
	}
	y.listCtx, y.listCtxCancel = context.WithCancel(context.Background())
	for _, list := range y.listeners {
		go y.acceptLoop(y.listCtx, list)
	}
	return
}

func (y *yamuxTransport) SetAccepter(accepter transport.Accepter) {
	y.accepter = accepter
}

func (y *yamuxTransport) Dial(ctx context.Context, addr string) (mc transport.MultiConn, err error) {
	dialTimeout := time.Duration(y.conf.DialTimeoutSec) * time.Second
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	cctx, sc, err := y.secure.SecureOutbound(ctx, conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	luc := connutil.NewLastUsageConn(sc)
	sess, err := yamux.Client(luc, y.yamuxConf)
	if err != nil {
		return
	}
	mc = &yamuxConn{
		ctx:     cctx,
		luConn:  luc,
		Session: sess,
	}
	return
}

func (y *yamuxTransport) acceptLoop(ctx context.Context, list net.Listener) {
	l := log.With(zap.String("localAddr", list.Addr().String()))
	l.Info("yamux listener started")
	defer func() {
		l.Debug("yamux listener stopped")
	}()
	for {
		conn, err := list.Accept()
		if err != nil {
			if isTemporary(err) {
				l.Debug("listener temporary accept error", zap.Error(err))
				select {
				case <-time.After(time.Second):
				case <-ctx.Done():
					return
				}
				continue
			}
			if err != net.ErrClosed {
				l.Error("listener closed with error", zap.Error(err))
			} else {
				l.Info("listener closed")
			}
			return
		}
		go y.accept(conn)
	}
}

func (y *yamuxTransport) accept(conn net.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(y.conf.DialTimeoutSec)*time.Second)
	defer cancel()
	cctx, sc, err := y.secure.SecureInbound(ctx, conn)
	if err != nil {
		log.Warn("incoming connection handshake error", zap.Error(err))
		return
	}
	luc := connutil.NewLastUsageConn(sc)
	sess, err := yamux.Server(luc, y.yamuxConf)
	if err != nil {
		log.Warn("incoming connection yamux session error", zap.Error(err))
		return
	}
	mc := &yamuxConn{
		ctx:     cctx,
		luConn:  luc,
		Session: sess,
	}
	if err = y.accepter.Accept(mc); err != nil {
		log.Warn("connection accept error", zap.Error(err))
	}
}

func (y *yamuxTransport) Close(ctx context.Context) (err error) {
	if y.listCtxCancel != nil {
		y.listCtxCancel()
	}
	for _, l := range y.listeners {
		_ = l.Close()
	}
	return
}
