package server

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/pool"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/secure"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"io"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"time"
)

const CName = "net/drpcserver"

var log = logger.NewNamed(CName)

func New() DRPCServer {
	return &drpcServer{Mux: drpcmux.New()}
}

type DRPCServer interface {
	app.ComponentRunnable
	drpc.Mux
}

type drpcServer struct {
	config     config.GrpcServer
	drpcServer *drpcserver.Server
	transport  secure.Service
	listeners  []secure.ContextListener
	pool       pool.Pool
	cancel     func()
	*drpcmux.Mux
}

func (s *drpcServer) Init(ctx context.Context, a *app.App) (err error) {
	s.config = a.MustComponent(config.CName).(*config.Config).GrpcServer
	s.transport = a.MustComponent(secure.CName).(secure.Service)
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	return nil
}

func (s *drpcServer) Name() (name string) {
	return CName
}

func (s *drpcServer) Run(ctx context.Context) (err error) {
	s.drpcServer = drpcserver.New(s.Mux)
	ctx, s.cancel = context.WithCancel(ctx)
	for _, addr := range s.config.ListenAddrs {
		tcpList, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		tlsList := s.transport.TLSListener(tcpList)
		go s.serve(ctx, tlsList)
	}
	return
}

func (s *drpcServer) serve(ctx context.Context, lis secure.ContextListener) {
	l := log.With(zap.String("localAddr", lis.Addr().String()))
	l.Info("drpc listener started")
	defer func() {
		l.Debug("drpc listener stopped")
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ctx, conn, err := lis.Accept(ctx)
		if err != nil {
			if isTemporary(err) {
				l.Debug("listener temporary accept error", zap.Error(err))
				t := time.NewTimer(500 * time.Millisecond)
				select {
				case <-t.C:
				case <-ctx.Done():
					return
				}
				continue
			}
			if _, ok := err.(secure.HandshakeError); ok {
				l.Warn("listener handshake error", zap.Error(err))
				continue
			}
			l.Error("listener accept error", zap.Error(err))
			return
		}
		go s.serveConn(ctx, conn)
	}
}

func (s *drpcServer) serveConn(ctx context.Context, conn net.Conn) {
	l := log.With(zap.String("remoteAddr", conn.RemoteAddr().String())).With(zap.String("localAddr", conn.LocalAddr().String()))
	l.Debug("connection opened")
	if err := s.drpcServer.ServeOne(ctx, conn); err != nil {
		if errs.Is(err, context.Canceled) || errs.Is(err, io.EOF) {
			l.Debug("connection closed")
		} else {
			l.Warn("serve connection error", zap.Error(err))
		}
	}
}

func (s *drpcServer) Close(ctx context.Context) (err error) {
	if s.cancel != nil {
		s.cancel()
	}
	for _, l := range s.listeners {
		if e := l.Close(); e != nil {
			log.Warn("close listener error", zap.Error(e))
		}
	}
	return
}
