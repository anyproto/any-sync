package server

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/secure"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"io"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"time"
)

type BaseDrpcServer struct {
	drpcServer *drpcserver.Server
	transport  secure.Service
	listeners  []secure.ContextListener
	cancel     func()
	*drpcmux.Mux
}

type DRPCHandlerWrapper func(handler drpc.Handler) drpc.Handler

func NewBaseDrpcServer() *BaseDrpcServer {
	return &BaseDrpcServer{Mux: drpcmux.New()}
}

func (s *BaseDrpcServer) Init(transport secure.Service) {
	s.transport = transport
}

func (s *BaseDrpcServer) Run(ctx context.Context, listenAddrs []string, wrapper DRPCHandlerWrapper) (err error) {
	s.drpcServer = drpcserver.New(wrapper(s.Mux))
	ctx, s.cancel = context.WithCancel(ctx)
	for _, addr := range listenAddrs {
		tcpList, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		tlsList := s.transport.TLSListener(tcpList)
		go s.serve(ctx, tlsList)
	}
	return
}

func (s *BaseDrpcServer) serve(ctx context.Context, lis secure.ContextListener) {
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

func (s *BaseDrpcServer) serveConn(ctx context.Context, conn net.Conn) {
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

func (s *BaseDrpcServer) Close(ctx context.Context) (err error) {
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
