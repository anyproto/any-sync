package main

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"time"
)

// req/rep	1000000 req for 1m56.663998462s (8571 per sec)
// stream	1000000 req for 1m25.54958362s (11689 per sec)

func main() {

	conf, err := config.NewFromFile("etc/config.yml")
	if err != nil {
		panic(err)
	}
	benchGrpc(conf)
	benchDrpc(conf)
}

func benchGrpc(conf *config.Config) {
	var opts []grpc.DialOption
	if conf.GrpcServer.TLS {
		creds, err := credentials.NewClientTLSFromFile(conf.GrpcServer.TLSCertFile, "127.0.0.1")
		if err != nil {
			log.Fatalf("Failed to create TLS credentials %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(conf.GrpcServer.ListenAddrs[0], opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := syncproto.NewAnytypeSyncClient(conn)

	stream, err := client.Ping(context.TODO())
	if err != nil {
		panic(err)
	}

	st := time.Now()
	n := 100000

	for i := 0; i < n; i++ {
		if err = stream.Send(&syncproto.PingRequest{
			Seq: int64(i),
		}); err != nil {
			panic(err)
		}
		_, err := stream.Recv()
		if err != nil {
			panic(err)
		}
	}
	dur := time.Since(st)
	fmt.Printf("%d req for %v (%d per sec)\n", n, dur, int(float64(n)/dur.Seconds()))
}

func benchDrpc(conf *config.Config) {
	rawconn, err := net.Dial("tcp", conf.GrpcServer.ListenAddrs[1])
	if err != nil {
		panic(err)
	}
	conn := drpcconn.New(rawconn)
	defer conn.Close()
	client := syncproto.NewDRPCAnytypeSyncClient(conn)

	stream, err := client.Ping(context.TODO())
	if err != nil {
		panic(err)
	}

	st := time.Now()
	n := 100000

	for i := 0; i < n; i++ {
		if err = stream.MsgSend(&syncproto.PingRequest{
			Seq: int64(i),
		}, enc{}); err != nil {
			panic(err)
		}
		msg := &syncproto.PingResponse{}
		err := stream.MsgRecv(msg, enc{})
		if err != nil {
			panic(err)
		}
	}
	dur := time.Since(st)
	fmt.Printf("%d req for %v (%d per sec)\n", n, dur, int(float64(n)/dur.Seconds()))
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
