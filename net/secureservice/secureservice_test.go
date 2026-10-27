package secureservice

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/testnodeconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"io"
	"net"
	"sync/atomic"
	"testing"
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

	cctx, err := fxC.SecureOutbound(ctx, cc, "node1.example.org")
	require.NoError(t, err)
	ctxPeerId, err := peer.CtxPeerId(cctx)
	require.NoError(t, err)
	assert.Equal(t, nc.GetAccountService(0).Account().PeerId, ctxPeerId)
	res := <-resCh
	require.NoError(t, res.err)
	peerId, err := peer.CtxPeerId(res.ctx)
	require.NoError(t, err)
	accId, err := peer.CtxIdentity(res.ctx)
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
	_, err := fxC.SecureOutbound(ctx, cc, "")
	require.Equal(t, handshake.ErrIncompatibleVersion, err)
	res := <-resCh
	require.Equal(t, handshake.ErrIncompatibleVersion, res.err)
}

func TestOutboundServerName(t *testing.T) {
	nc := testnodeconf.GenNodeConfig(1)
	for _, tc := range []struct {
		name       string
		conf       app.Component
		serverName string
		want       string
	}{
		{name: "sent by default", conf: nc, serverName: "node1.example.org", want: "node1.example.org"},
		{name: "ip address never sent", conf: nc, serverName: "127.0.0.1", want: ""},
		{name: "disableSni", conf: &secureConf{Config: nc, secure: Config{DisableSNI: true}}, serverName: "node1.example.org", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t, tc.conf, nc.GetAccountService(0), 1, []uint32{1})
			defer fx.Finish(t)

			t.Run("SecureOutbound", func(t *testing.T) {
				assert.Equal(t, tc.want, sentServerName(t, func(conn net.Conn) {
					_, _ = fx.SecureOutbound(ctx, conn, tc.serverName)
				}))
			})
			t.Run("TlsConfig", func(t *testing.T) {
				conf, _, err := fx.TlsConfig(tc.serverName)
				require.NoError(t, err)
				assert.Equal(t, tc.want, sentServerName(t, func(conn net.Conn) {
					_ = tls.Client(conn, conf).HandshakeContext(ctx)
				}))
			})
		})
	}
}

func TestSecureOutboundClosesConnOnTLSFailure(t *testing.T) {
	nc := testnodeconf.GenNodeConfig(1)
	fx := newFixture(t, nc, nc.GetAccountService(0), 1, []uint32{1})
	defer fx.Finish(t)
	sc, cc := net.Pipe()
	defer sc.Close()
	// answer the ClientHello with plaintext HTTP, as a misbehaving proxy would
	go func() { _, _ = io.Copy(io.Discard, sc) }()
	go func() { _, _ = sc.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n")) }()

	conn := &closeRecordingConn{Conn: cc}
	_, err := fx.SecureOutbound(ctx, conn, "node1.example.org")
	require.Error(t, err)
	assert.True(t, conn.closed.Load(), "conn must be closed when the TLS handshake fails")
}

type closeRecordingConn struct {
	net.Conn
	closed atomic.Bool
}

func (c *closeRecordingConn) Close() error {
	c.closed.Store(true)
	return c.Conn.Close()
}

// sentServerName returns the SNI of the ClientHello that dial writes, aborting
// the handshake as soon as the hello is read
func sentServerName(t *testing.T, dial func(conn net.Conn)) string {
	sc, cc := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		dial(cc)
		_ = cc.Close()
	}()
	var (
		gotHello   bool
		serverName string
	)
	_ = tls.Server(sc, &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			gotHello, serverName = true, hello.ServerName
			return nil, errors.New("hello captured")
		},
	}).HandshakeContext(ctx)
	_ = sc.Close()
	<-done
	require.True(t, gotHello, "no ClientHello received")
	return serverName
}

type secureConf struct {
	*testnodeconf.Config
	secure Config
}

func (c *secureConf) GetSecureService() Config { return c.secure }

func newFixture(t *testing.T, conf app.Component, acc accountservice.Service, protoVersion uint32, cv []uint32) *fixture {
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
	fx.a.Register(fx.acc).Register(conf).Register(fx.mockNodeConf).Register(fx.secureService)
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
