package peer

import (
	"context"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anyproto/any-sync/net/transport/mock_transport"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"testing"
	"time"
)

var ctx = context.Background()

func TestPeer_AcquireDrpcConn(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	in, out := net.Pipe()
	defer out.Close()
	fx.mc.EXPECT().Open(gomock.Any()).Return(in, nil)
	dc, err := fx.AcquireDrpcConn(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, dc)
	defer dc.Close()

	assert.Len(t, fx.active, 1)
	assert.Len(t, fx.inactive, 0)

	fx.ReleaseDrpcConn(dc)

	assert.Len(t, fx.active, 0)
	assert.Len(t, fx.inactive, 1)

	dc, err = fx.AcquireDrpcConn(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, dc)
	assert.Len(t, fx.active, 1)
	assert.Len(t, fx.inactive, 0)
}

func TestPeerAccept(t *testing.T) {
	fx := newFixture(t, "p1")
	defer fx.finish()
	in, out := net.Pipe()
	defer out.Close()

	var outHandshakeCh = make(chan error)
	go func() {
		outHandshakeCh <- handshake.OutgoingProtoHandshake(ctx, out, handshakeproto.ProtoType_DRPC)
	}()
	fx.acceptCh <- acceptedConn{conn: in}
	cn := <-fx.testCtrl.serveConn
	assert.Equal(t, in, cn)
	assert.NoError(t, <-outHandshakeCh)
}

func TestPeer_TryClose(t *testing.T) {
	t.Run("ttl", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		lu := time.Now()
		fx.mc.EXPECT().LastUsage().Return(lu)
		res, err := fx.TryClose(time.Second)
		require.NoError(t, err)
		assert.False(t, res)
	})
	t.Run("close", func(t *testing.T) {
		fx := newFixture(t, "p1")
		defer fx.finish()
		lu := time.Now().Add(-time.Second * 2)
		fx.mc.EXPECT().LastUsage().Return(lu)
		res, err := fx.TryClose(time.Second)
		require.NoError(t, err)
		assert.True(t, res)
	})
}

type acceptedConn struct {
	conn net.Conn
	err  error
}

func newFixture(t *testing.T, peerId string) *fixture {
	fx := &fixture{
		ctrl:     gomock.NewController(t),
		acceptCh: make(chan acceptedConn),
		testCtrl: newTesCtrl(),
	}
	fx.mc = mock_transport.NewMockMultiConn(fx.ctrl)
	ctx := CtxWithPeerId(context.Background(), peerId)
	fx.mc.EXPECT().Context().Return(ctx).AnyTimes()
	fx.mc.EXPECT().Accept().DoAndReturn(func() (net.Conn, error) {
		ac := <-fx.acceptCh
		return ac.conn, ac.err
	}).AnyTimes()
	fx.mc.EXPECT().Close().AnyTimes()
	p, err := NewPeer(fx.mc, fx.testCtrl)
	require.NoError(t, err)
	fx.peer = p.(*peer)
	return fx
}

type fixture struct {
	*peer
	ctrl     *gomock.Controller
	mc       *mock_transport.MockMultiConn
	acceptCh chan acceptedConn
	testCtrl *testCtrl
}

func (fx *fixture) finish() {
	fx.testCtrl.close()
	fx.ctrl.Finish()
}

func newTesCtrl() *testCtrl {
	return &testCtrl{closeCh: make(chan struct{}), serveConn: make(chan net.Conn, 10)}
}

type testCtrl struct {
	serveConn chan net.Conn
	closeCh   chan struct{}
}

func (t *testCtrl) ServeConn(ctx context.Context, conn net.Conn) (err error) {
	t.serveConn <- conn
	<-t.closeCh
	return io.EOF
}

func (t *testCtrl) close() {
	close(t.closeCh)
}
