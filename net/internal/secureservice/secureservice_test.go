package secureservice

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	peer2 "github.com/anyproto/any-sync/net/peer"
	handshake2 "github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/testnodeconf"
)

var ctx = context.Background()

func TestHandshake(t *testing.T) {
	nc := testnodeconf.GenNodeConfig(2)
	fxS := newFixture(t, nc, nc.GetAccountService(0), 1, []uint32{1})
	defer fxS.Finish(t)
	sc, cc := net.Pipe()

	type acceptRes struct {
		ctx  context.Context
		conn net.Conn
		err  error
	}
	resCh := make(chan acceptRes)
	go func() {
		var ar acceptRes
		ar.ctx, ar.err = fxS.SecureInbound(ctx, sc)
		resCh <- ar
	}()

	fxC := newFixture(t, nc, nc.GetAccountService(1), 1, []uint32{1})
	defer fxC.Finish(t)

	cctx, err := fxC.SecureOutbound(ctx, cc)
	require.NoError(t, err)
	ctxPeerId, err := peer2.CtxPeerId(cctx)
	require.NoError(t, err)
	assert.Equal(t, nc.GetAccountService(0).Account().PeerId, ctxPeerId)
	res := <-resCh
	require.NoError(t, res.err)
	peerId, err := peer2.CtxPeerId(res.ctx)
	require.NoError(t, err)
	accId, err := peer2.CtxIdentity(res.ctx)
	require.NoError(t, err)
	marshalledId, _ := nc.GetAccountService(1).Account().SignKey.GetPublic().Marshall()
	assert.Equal(t, nc.GetAccountService(1).Account().PeerId, peerId)
	assert.Equal(t, marshalledId, accId)
}

func TestHandshakeIncompatibleVersion(t *testing.T) {
	nc := testnodeconf.GenNodeConfig(2)
	fxS := newFixture(t, nc, nc.GetAccountService(0), 1, []uint32{0, 1})
	defer fxS.Finish(t)
	sc, cc := net.Pipe()

	type acceptRes struct {
		ctx  context.Context
		conn net.Conn
		err  error
	}
	resCh := make(chan acceptRes)
	go func() {
		var ar acceptRes
		ar.ctx, ar.err = fxS.SecureInbound(ctx, sc)
		resCh <- ar
	}()
	fxC := newFixture(t, nc, nc.GetAccountService(1), 2, []uint32{2, 3})
	defer fxC.Finish(t)
	_, err := fxC.SecureOutbound(ctx, cc)
	require.Equal(t, handshake2.ErrIncompatibleVersion, err)
	res := <-resCh
	require.Equal(t, handshake2.ErrIncompatibleVersion, res.err)
}

func newFixture(t *testing.T, nc *testnodeconf.Config, acc accountservice.Service, protoVersion uint32, cv []uint32) *fixture {
	fx := &fixture{
		ctrl:          gomock.NewController(t),
		secureService: New().(*secureService),
		acc:           acc,
		a:             new(app.App),
	}
	fx.secureService.protoVersion = protoVersion
	fx.secureService.compatibleVersions = cv
	fx.mockNodeConf = mock_nodeconf.NewMockService(fx.ctrl)
	fx.mockNodeConf.EXPECT().Init(gomock.Any())
	fx.mockNodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.mockNodeConf.EXPECT().Run(ctx)
	fx.mockNodeConf.EXPECT().Close(ctx)
	fx.mockNodeConf.EXPECT().NodeTypes(gomock.Any()).Return([]nodeconf.NodeType{nodeconf.NodeTypeTree}).AnyTimes()
	fx.a.Register(fx.acc).Register(nc).Register(fx.mockNodeConf).Register(fx.secureService)
	require.NoError(t, fx.a.Start(ctx))
	return fx
}

type fixture struct {
	*secureService
	a            *app.App
	acc          accountservice.Service
	ctrl         *gomock.Controller
	mockNodeConf *mock_nodeconf.MockService
}

func (fx *fixture) Finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
}
