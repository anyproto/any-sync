package debugserver

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/rpc"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"storj.io/drpc/drpcstream"
	"storj.io/drpc/drpcwire"
)

const CName = "net.rpc.debugserver"

func New() DebugServer {
	return &debugServer{}
}

type DebugServer interface {
	app.ComponentRunnable
	drpc.Mux
}

type debugServer struct {
	drpcServer *drpcserver.Server
	*drpcmux.Mux
	drpcConf     rpc.Config
	config       Config
	runCtx       context.Context
	runCtxCancel context.CancelFunc
}

func (d *debugServer) Init(a *app.App) (err error) {
	d.drpcConf = a.MustComponent("config").(rpc.ConfigGetter).GetDrpc()
	d.config = a.MustComponent("config").(configGetter).GetDebugServer()
	d.Mux = drpcmux.New()
	bufSize := d.drpcConf.Stream.MaxMsgSizeMb * (1 << 20)
	d.drpcServer = drpcserver.NewWithOptions(d, drpcserver.Options{Manager: drpcmanager.Options{
		Reader: drpcwire.ReaderOptions{MaximumBufferSize: bufSize},
		Stream: drpcstream.Options{MaximumBufferSize: bufSize},
	}})
	return nil
}

func (d *debugServer) Name() (name string) {
	return CName
}

func (d *debugServer) Run(ctx context.Context) (err error) {
	if d.config.ListenAddr == "" {
		return
	}
	lis, err := net.Listen("tcp", d.config.ListenAddr)
	if err != nil {
		return
	}
	d.runCtx, d.runCtxCancel = context.WithCancel(context.Background())
	go d.drpcServer.Serve(d.runCtx, lis)
	return
}

func (d *debugServer) Close(ctx context.Context) (err error) {
	if d.runCtx != nil {
		d.runCtxCancel()
	}
	return nil
}
