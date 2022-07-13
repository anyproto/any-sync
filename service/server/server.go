package server

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"net"
	"time"
)

var log = logger.NewNamed("server")

const CName = "server"

func New() *Server {
	return &Server{}
}

type Server struct {
	config     config.GrpcServer
	grpcServer *grpc.Server
}

func (s *Server) Init(ctx context.Context, a *app.App) (err error) {
	s.config = a.MustComponent(config.CName).(*config.Config).GrpcServer
	return nil
}

func (s *Server) Name() (name string) {
	return CName
}

func (s *Server) Run(ctx context.Context) (err error) {
	lis, err := net.Listen("tcp", s.config.ListenAddrs[0])
	if err != nil {
		return err
	}
	var opts []grpc.ServerOption
	if s.config.TLS {
		creds, err := credentials.NewServerTLSFromFile(s.config.TLSCertFile, s.config.TLSKeyFile)
		if err != nil {
			return err
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	s.grpcServer = grpc.NewServer(opts...)

	syncproto.RegisterAnytypeSyncServer(s.grpcServer, s)

	var errCh = make(chan error)
	go func() {
		errCh <- s.grpcServer.Serve(lis)
	}()
	select {
	case <-time.After(time.Second / 4):
	case err = <-errCh:
	}

	log.Sugar().Infof("server started at: %v", s.config.ListenAddrs[0])
	return
}

func (s *Server) Ping(stream syncproto.AnytypeSync_PingServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := stream.Send(&syncproto.PingResponse{
			Seq: in.Seq,
		}); err != nil {
			return err
		}
	}
}

func (s *Server) Close(ctx context.Context) (err error) {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	return
}
