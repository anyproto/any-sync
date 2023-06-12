package nettest

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/coordinator/coordinatorclient"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/coordinator/nodeconfsource"
	"github.com/anyproto/any-sync/net/peerservice"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport/yamux"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/nodeconfstore"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"net/http"
	_ "net/http/pprof"
	"testing"
	"time"
)

var ctx = context.Background()

func init() {
	go http.ListenAndServe(":6061", nil)
}

func TestNet(t *testing.T) {
	l := logger.NewNamed("test")
	pl := pool.New()
	a := new(app.App)
	a.Register(&accounttest.AccountTestService{}).
		Register(pl).
		Register(peerservice.New()).
		Register(yamux.New()).
		Register(nodeconf.New()).
		Register(nodeconfstore.New()).
		Register(nodeconfsource.New()).
		Register(server.New()).
		Register(secureservice.New()).
		Register(coordinatorclient.New()).
		Register(&testConfig{})
	require.NoError(t, a.Start(ctx))
	defer a.Close(ctx)

	p, err := pl.Get(ctx, "12D3KooWB4hmEo7YAdWzAaFpjyk4npkcwrPm2kRigsWu3MP9Xdmg")
	require.NoError(t, err)

	for i := 0; i < 1000000; i++ {
		dc, err := p.AcquireDrpcConn(ctx)
		require.NoError(t, err)
		cl := coordinatorproto.NewDRPCCoordinatorClient(dc)
		time.Sleep(time.Second)
		res, err := cl.NetworkConfiguration(ctx, &coordinatorproto.NetworkConfigurationRequest{})
		if err != nil {
			l.Warn("req error", zap.Error(err))
		} else {

			l.Info("req success", zap.String("nid", res.NetworkId))
		}
		p.ReleaseDrpcConn(dc)
	}

}

type testConfig struct {
}

func (t *testConfig) GetDrpc() rpc.Config {
	return rpc.Config{
		Stream: rpc.StreamConfig{
			MaxMsgSizeMb: 1,
		},
	}
}

func (t *testConfig) GetNodeConfStorePath() string {
	return "/tmp"
}

func (t *testConfig) GetNodeConf() nodeconf.Configuration {
	return nodeconf.Configuration{
		Id:        "",
		NetworkId: "",
		Nodes: []nodeconf.Node{
			{
				PeerId:    "12D3KooWB4hmEo7YAdWzAaFpjyk4npkcwrPm2kRigsWu3MP9Xdmg",
				Addresses: []string{"ec2-3-65-13-48.eu-central-1.compute.amazonaws.com:443"},
				Types:     []nodeconf.NodeType{nodeconf.NodeTypeTree},
			},
		},
		CreationTime: time.Now(),
	}
}

func (t *testConfig) GetYamux() yamux.Config {
	return yamux.Config{
		WriteTimeoutSec: 10,
		DialTimeoutSec:  10,
	}
}

func (t *testConfig) Init(a *app.App) (err error) {
	return nil
}

func (t *testConfig) Name() (name string) {
	return "config"
}
