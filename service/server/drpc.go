package server

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"io"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"time"
)

const CNameDRPC = "serverDrpc"

func NewDRPC() *ServerDrpc {
	return &ServerDrpc{}
}

type ServerDrpc struct {
	config         config.GrpcServer
	grpcServerDrpc *drpcserver.Server
	cancel         func()
}

func (s *ServerDrpc) Init(ctx context.Context, a *app.App) (err error) {
	s.config = a.MustComponent(config.CName).(*config.Config).GrpcServer
	return nil
}

func (s *ServerDrpc) Name() (name string) {
	return CNameDRPC
}

func (s *ServerDrpc) Run(ctx context.Context) (err error) {
	m := drpcmux.New()
	lis, err := net.Listen("tcp", s.config.ListenAddrs[1])
	if err != nil {
		return err
	}
	err = syncproto.DRPCRegisterAnytypeSync(m, s)
	if err != nil {
		return err
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.grpcServerDrpc = drpcserver.New(m)
	var errCh = make(chan error)
	go func() {
		errCh <- s.grpcServerDrpc.Serve(ctx, lis)
	}()
	select {
	case <-time.After(time.Second / 4):
	case err = <-errCh:
	}
	log.Sugar().Infof("drpc server started at: %v", s.config.ListenAddrs[1])
	return
}

func (s *ServerDrpc) Ping(stream syncproto.DRPCAnytypeSync_PingStream) error {
	for {
		var in = &syncproto.PingRequest{}
		err := stream.MsgRecv(in, enc{})
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := stream.MsgSend(&syncproto.PingResponse{
			Seq: in.Seq,
		}, enc{}); err != nil {
			return err
		}
	}
}

func (s *ServerDrpc) Close(ctx context.Context) (err error) {
	if s.cancel != nil {
		s.cancel()
	}
	return
}

type enc struct {
}

func (e enc) Marshal(msg drpc.Message) ([]byte, error) {
	return msg.(interface {
		Marshal() ([]byte, error)
	}).Marshal()
}

func (e enc) Unmarshal(buf []byte, msg drpc.Message) error {
	return msg.(interface {
		Unmarshal(buf []byte) error
	}).Unmarshal(buf)
}
