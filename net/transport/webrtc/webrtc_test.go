//go:build !js

package webrtc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	netpeer "github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/testutil/testnodeconf"
)

var ctx = context.Background()

func TestWebRTCTransport_Dial(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	// Dial with expected peerId in context
	serverPeerId := fxS.acc.Account().PeerId
	dialCtx := netpeer.CtxWithExpectedPeerId(ctx, serverPeerId)
	signalAddr := fmt.Sprintf("127.0.0.1:%d", fxS.SignalPort())

	mcC, err := fxC.Dial(dialCtx, signalAddr)
	require.NoError(t, err)

	var mcS transport.MultiConn
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout waiting for server accept")
	}

	// Verify peerId in server's connection context
	connPeerId, err := netpeer.CtxPeerId(mcS.Context())
	require.NoError(t, err)
	assert.Equal(t, fxC.acc.Account().PeerId, connPeerId)

	// Exchange data over sub-connections
	var (
		sData     string
		acceptErr error
		copyErr   error
		done      = make(chan struct{})
	)

	go func() {
		defer close(done)
		conn, serr := mcS.Accept()
		if serr != nil {
			acceptErr = serr
			return
		}
		buf := bytes.NewBuffer(nil)
		_, copyErr = io.Copy(buf, conn)
		sData = buf.String()
	}()

	conn, err := mcC.Open(ctx)
	require.NoError(t, err)
	data := "hello webrtc"
	_, err = conn.Write([]byte(data))
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	<-done

	assert.NoError(t, acceptErr)
	assert.Equal(t, data, sData)
	assert.NoError(t, copyErr)
}

func TestWebRTCTransport_MultipleStreams(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	serverPeerId := fxS.acc.Account().PeerId
	dialCtx := netpeer.CtxWithExpectedPeerId(ctx, serverPeerId)
	signalAddr := fmt.Sprintf("127.0.0.1:%d", fxS.SignalPort())

	mcC, err := fxC.Dial(dialCtx, signalAddr)
	require.NoError(t, err)

	var mcS transport.MultiConn
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout")
	}

	numStreams := 5
	var wg sync.WaitGroup

	// Server side: accept and echo
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numStreams; i++ {
			conn, err := mcS.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	// Client side: open streams and exchange data
	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn, err := mcC.Open(ctx)
			if err != nil {
				return
			}
			defer conn.Close()

			msg := fmt.Sprintf("stream-%d", idx)
			conn.Write([]byte(msg))

			buf := make([]byte, 64)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			assert.Equal(t, msg, string(buf[:n]))
		}(i)
	}

	wg.Wait()
}

func TestWebRTCTransport_Addr(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	serverPeerId := fxS.acc.Account().PeerId
	dialCtx := netpeer.CtxWithExpectedPeerId(ctx, serverPeerId)
	signalAddr := fmt.Sprintf("127.0.0.1:%d", fxS.SignalPort())

	mcC, err := fxC.Dial(dialCtx, signalAddr)
	require.NoError(t, err)

	// Verify address format
	assert.Contains(t, mcC.Addr(), "webrtc://")

	var mcS transport.MultiConn
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout")
	}
	assert.Contains(t, mcS.Addr(), "webrtc://")
}

func TestWebRTCTransport_Close(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	serverPeerId := fxS.acc.Account().PeerId
	dialCtx := netpeer.CtxWithExpectedPeerId(ctx, serverPeerId)
	signalAddr := fmt.Sprintf("127.0.0.1:%d", fxS.SignalPort())

	mcC, err := fxC.Dial(dialCtx, signalAddr)
	require.NoError(t, err)

	select {
	case <-fxS.accepter.mcs:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout")
	}

	assert.False(t, mcC.IsClosed())
	err = mcC.Close()
	assert.NoError(t, err)

	// Wait for CloseChan to be notified
	select {
	case <-mcC.CloseChan():
	case <-time.After(time.Second * 5):
		require.Fail(t, "CloseChan not signaled")
	}

	assert.True(t, mcC.IsClosed())
}

// Test fixtures following the same pattern as quic_test.go

type fixture struct {
	*webrtcTransport
	a            *app.App
	ctrl         *gomock.Controller
	mockNodeConf *mock_nodeconf.MockService
	acc          *accounttest.AccountTestService
	accepter     *testAccepter
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{
		webrtcTransport: New().(*webrtcTransport),
		ctrl:            gomock.NewController(t),
		acc:             &accounttest.AccountTestService{},
		accepter:        &testAccepter{mcs: make(chan transport.MultiConn, 100)},
		a:               new(app.App),
	}

	fx.mockNodeConf = mock_nodeconf.NewMockService(fx.ctrl)
	fx.mockNodeConf.EXPECT().Init(gomock.Any())
	fx.mockNodeConf.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	fx.mockNodeConf.EXPECT().Run(ctx)
	fx.mockNodeConf.EXPECT().Close(ctx)
	fx.mockNodeConf.EXPECT().NodeTypes(gomock.Any()).Return([]nodeconf.NodeType{nodeconf.NodeTypeTree}).AnyTimes()

	fx.a.Register(fx.acc).
		Register(newTestConf()).
		Register(fx.mockNodeConf).
		Register(secureservice.New()).
		Register(fx.webrtcTransport).
		Register(fx.accepter)
	require.NoError(t, fx.a.Start(ctx))
	return fx
}

func (fx *fixture) finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()
}

func newTestConf() *testConf {
	return &testConf{testnodeconf.GenNodeConfig(1)}
}

type testConf struct {
	*testnodeconf.Config
}

func (c *testConf) GetWebRTC() Config {
	return Config{
		ListenAddrs:     []string{"127.0.0.1:0"},
		SignalPort:      0,
		WriteTimeoutSec: 10,
		DialTimeoutSec:  30,
	}
}

type testAccepter struct {
	err error
	mcs chan transport.MultiConn
}

func (t *testAccepter) Accept(mc transport.MultiConn) (err error) {
	t.mcs <- mc
	return t.err
}

func (t *testAccepter) Init(a *app.App) (err error) {
	a.MustComponent(CName).(transport.Transport).SetAccepter(t)
	return nil
}

func (t *testAccepter) Name() (name string) { return "testAccepter" }

// Verify that the testAccepter is not called before the signal port is available
func TestWebRTCTransport_SignalPort(t *testing.T) {
	fx := newFixture(t)
	defer fx.finish(t)

	port := fx.SignalPort()
	assert.Greater(t, port, 0)

	// Verify signal endpoint is reachable
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, signalPath))
	// Should get 405 Method Not Allowed (GET vs POST)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 405, resp.StatusCode)
}
