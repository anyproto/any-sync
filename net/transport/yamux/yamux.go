package yamux

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/hashicorp/yamux"
	"go.uber.org/zap"
)

const CName = "net.transport.yamux"

var log = logger.NewNamed(CName)

func New() Yamux {
	return new(yamuxTransport)
}

// Yamux implements transport.Transport with tcp+yamux
type Yamux interface {
	transport.Transport
	AddListener(lis net.Listener)
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
	mu            sync.Mutex
}

func (y *yamuxTransport) Init(a *app.App) (err error) {
	y.secure = a.MustComponent(secureservice.CName).(secureservice.SecureService)
	y.conf = a.MustComponent("config").(configGetter).GetYamux()
	if y.conf.DialTimeoutSec <= 0 {
		y.conf.DialTimeoutSec = 10
	}
	if y.conf.WriteTimeoutSec <= 0 {
		y.conf.WriteTimeoutSec = 10
	}

	y.yamuxConf = yamux.DefaultConfig()
	if y.conf.KeepAlivePeriodSec < 0 {
		y.yamuxConf.EnableKeepAlive = false
	} else {
		y.yamuxConf.EnableKeepAlive = true
		if y.conf.KeepAlivePeriodSec != 0 {
			y.yamuxConf.KeepAliveInterval = time.Duration(y.conf.KeepAlivePeriodSec) * time.Second
		}
	}
	y.yamuxConf.StreamOpenTimeout = time.Duration(y.conf.DialTimeoutSec) * time.Second
	y.yamuxConf.ConnectionWriteTimeout = time.Duration(y.conf.WriteTimeoutSec) * time.Second
	y.listCtx, y.listCtxCancel = context.WithCancel(context.Background())
	return
}

func (y *yamuxTransport) Name() string {
	return CName
}

func (y *yamuxTransport) Run(ctx context.Context) (err error) {
	if y.accepter == nil {
		return fmt.Errorf("can't run service without accepter")
	}
	y.mu.Lock()
	defer y.mu.Unlock()
	for _, listAddr := range y.conf.ListenAddrs {
		list, err := net.Listen("tcp", listAddr)
		if err != nil {
			return err
		}
		y.listeners = append(y.listeners, list)
	}
	for _, list := range y.listeners {
		go y.acceptLoop(y.listCtx, list)
	}
	return
}

func (y *yamuxTransport) SetAccepter(accepter transport.Accepter) {
	y.accepter = accepter
}

func (y *yamuxTransport) AddListener(lis net.Listener) {
	y.mu.Lock()
	defer y.mu.Unlock()
	y.listeners = append(y.listeners, lis)
	go y.acceptLoop(y.listCtx, lis)
}

func (y *yamuxTransport) Dial(ctx context.Context, addr string) (mc transport.MultiConn, err error) {
	dialTimeout := time.Duration(y.conf.DialTimeoutSec) * time.Second
	
	// Setup abort mechanism for context cancellation
	done := make(chan struct{})
	
	dialer := &net.Dialer{
		Timeout: dialTimeout,
		Control: controlFunc(ctx, done),
	}
	
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	
	// Signal the abortor goroutine to exit
	close(done)
	
	if err != nil {
		return nil, err
	}
	
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	cctx, err := y.secure.SecureOutbound(ctx, conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	luc := connutil.NewLastUsageConn(connutil.NewTimeout(conn, time.Duration(y.conf.WriteTimeoutSec)*time.Second))
	sess, err := yamux.Client(luc, y.yamuxConf)
	if err != nil {
		return
	}
	mc = NewMultiConn(cctx, luc, addr, sess)
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
	cctx, err := y.secure.SecureInbound(ctx, conn)
	if err != nil {
		log.Info("incoming connection handshake error", zap.Error(err), zap.String("remoteAddr", conn.RemoteAddr().String()))
		return
	}
	luc := connutil.NewLastUsageConn(connutil.NewTimeout(conn, time.Duration(y.conf.WriteTimeoutSec)*time.Second))
	sess, err := yamux.Server(luc, y.yamuxConf)
	if err != nil {
		log.Info("incoming connection yamux session error", zap.Error(err), zap.String("remoteAddr", conn.RemoteAddr().String()))
		return
	}
	mc := NewMultiConn(cctx, luc, conn.RemoteAddr().String(), sess)
	if err = y.accepter.Accept(mc); err != nil {
		log.Info("connection accept error", zap.Error(err), zap.String("remoteAddr", conn.RemoteAddr().String()))
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
