package server

import (
	"context"
	"github.com/anytypeio/any-sync/net/secureservice"
	"github.com/libp2p/go-libp2p/core/sec"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"io"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"storj.io/drpc/drpcwire"
	"time"
)

type BaseDrpcServer struct {
	drpcServer *drpcserver.Server
	transport  secureservice.SecureService
	listeners  []net.Listener
	handshake  func(conn net.Conn) (cCtx context.Context, sc sec.SecureConn, err error)
	cancel     func()
	*drpcmux.Mux
}

type DRPCHandlerWrapper func(handler drpc.Handler) drpc.Handler

type Params struct {
	BufferSizeMb  int
	ListenAddrs   []string
	Wrapper       DRPCHandlerWrapper
	TimeoutMillis int
}

func NewBaseDrpcServer() *BaseDrpcServer {
	return &BaseDrpcServer{Mux: drpcmux.New()}
}

func (s *BaseDrpcServer) Run(ctx context.Context, params Params) (err error) {
	s.drpcServer = drpcserver.NewWithOptions(params.Wrapper(s.Mux), drpcserver.Options{Manager: drpcmanager.Options{
		Reader: drpcwire.ReaderOptions{MaximumBufferSize: params.BufferSizeMb * (1 << 20)},
	}})
	ctx, s.cancel = context.WithCancel(ctx)
	for _, addr := range params.ListenAddrs {
		list, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		s.listeners = append(s.listeners, list)
		go s.serve(ctx, list)
	}
	return
}

func (s *BaseDrpcServer) serve(ctx context.Context, lis net.Listener) {
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
		conn, err := lis.Accept()
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
			l.Error("listener accept error", zap.Error(err))
			return
		}
		go s.serveConn(conn)
	}
}

func (s *BaseDrpcServer) serveConn(conn net.Conn) {
	l := log.With(zap.String("remoteAddr", conn.RemoteAddr().String())).With(zap.String("localAddr", conn.LocalAddr().String()))
	var (
		ctx = context.Background()
		err error
	)
	if s.handshake != nil {
		ctx, conn, err = s.handshake(conn)
		if err != nil {
			l.Info("handshake error", zap.Error(err))
			return
		}
	}

	l.Debug("connection opened")
	if err := s.drpcServer.ServeOne(ctx, conn); err != nil {
		if errs.Is(err, context.Canceled) || errs.Is(err, io.EOF) {
			l.Debug("connection closed")
		} else {
			l.Warn("serve connection error", zap.Error(err))
		}
	}
}

func (s *BaseDrpcServer) ListenAddrs() (addrs []net.Addr) {
	for _, list := range s.listeners {
		addrs = append(addrs, list.Addr())
	}
	return
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
