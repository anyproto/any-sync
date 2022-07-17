package drpcserver

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/transport"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"io"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcserver"
	"strings"
	"time"
)

var log = logger.NewNamed("drpcserver")

const CName = "drpcserver"

func New() DRPCServer {
	return &drpcServer{}
}

type DRPCServer interface {
	app.ComponentRunnable
}

type drpcServer struct {
	config     config.GrpcServer
	drpcServer *drpcserver.Server
	transport  transport.Service
	listeners  []transport.ContextListener
	cancel     func()
}

func (s *drpcServer) Init(ctx context.Context, a *app.App) (err error) {
	s.config = a.MustComponent(config.CName).(*config.Config).GrpcServer
	s.transport = a.MustComponent(transport.CName).(transport.Service)
	return nil
}

func (s *drpcServer) Name() (name string) {
	return CName
}

func (s *drpcServer) Run(ctx context.Context) (err error) {
	s.drpcServer = drpcserver.New(s)
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

func (s *drpcServer) serve(ctx context.Context, lis transport.ContextListener) {
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
			if _, ok := err.(transport.HandshakeError); ok {
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
		if err == context.Canceled || strings.Contains(err.Error(), "EOF") {
			l.Debug("connection closed")
		} else {
			l.Warn("serve connection error", zap.Error(err))
		}
	}
}

func (s *drpcServer) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	ctx := stream.Context()
	sc, err := transport.CtxSecureConn(ctx)
	if err != nil {
		return
	}
	l := log.With(zap.String("peer", sc.RemotePeer().String()))
	l.Info("stream opened")
	defer func() {
		l.Info("stream closed", zap.Error(err))
	}()
	for {
		msg := &syncproto.SyncMessage{}
		if err = stream.MsgRecv(msg, enc{}); err != nil {
			if err == io.EOF {
				return
			}
		}
		//log.Debug("receive msg", zap.Int("seq", int(msg.Seq)))
		if err = stream.MsgSend(msg, enc{}); err != nil {
			return
		}
	}
	return nil
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

type enc struct{}

func (e enc) Marshal(msg drpc.Message) ([]byte, error) {
	return msg.(proto.Marshaler).Marshal()
}

func (e enc) Unmarshal(buf []byte, msg drpc.Message) error {
	return msg.(proto.Unmarshaler).Unmarshal(buf)
}
