package secureservice

import (
	"context"
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/testutil/testnodeconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

var ctx = context.Background()

func TestHandshake(t *testing.T) {
	nc := testnodeconf.GenNodeConfig(2)
	fxS := newFixture(t, nc, nc.GetAccountService(0))
	defer fxS.Finish(t)

	tl := &testListener{conn: make(chan net.Conn, 1)}
	defer tl.Close()

	list := fxS.TLSListener(tl, 1000, true)
	type acceptRes struct {
		ctx  context.Context
		conn net.Conn
		err  error
	}
	resCh := make(chan acceptRes)
	go func() {
		var ar acceptRes
		ar.ctx, ar.conn, ar.err = list.Accept(ctx)
		resCh <- ar
	}()

	fxC := newFixture(t, nc, nc.GetAccountService(1))
	defer fxC.Finish(t)

	sc, cc := net.Pipe()
	tl.conn <- sc
	secConn, err := fxC.TLSConn(ctx, cc)
	require.NoError(t, err)
	assert.Equal(t, nc.GetAccountService(0).Account().PeerId, secConn.RemotePeer().String())
	res := <-resCh
	require.NoError(t, res.err)
	peerId, err := peer.CtxPeerId(res.ctx)
	require.NoError(t, err)
	accId, err := peer.CtxIdentity(res.ctx)
	require.NoError(t, err)
	assert.Equal(t, nc.GetAccountService(1).Account().PeerId, peerId)
	assert.Equal(t, nc.GetAccountService(1).Account().Identity, accId)
}

func newFixture(t *testing.T, nc *testnodeconf.Config, acc accountservice.Service) *fixture {
	fx := &fixture{
		secureService: New().(*secureService),
		acc:           acc,
		a:             new(app.App),
	}

	fx.a.Register(fx.acc).Register(fx.secureService).Register(nodeconf.New()).Register(nc)
	require.NoError(t, fx.a.Start(ctx))
	return fx
}

type fixture struct {
	*secureService
	a   *app.App
	acc accountservice.Service
}

func (fx *fixture) Finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
}

type testListener struct {
	conn chan net.Conn
}

func (t *testListener) Accept() (net.Conn, error) {
	conn, ok := <-t.conn
	if !ok {
		return nil, net.ErrClosed
	}
	return conn, nil
}

func (t *testListener) Close() error {
	close(t.conn)
	return nil
}

func (t *testListener) Addr() net.Addr {
	return nil
}
