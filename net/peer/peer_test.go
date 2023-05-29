package peer

import (
	"context"
	"github.com/anyproto/any-sync/net/transport/mock_transport"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
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

func newFixture(t *testing.T, peerId string) *fixture {
	fx := &fixture{
		ctrl: gomock.NewController(t),
	}
	fx.mc = mock_transport.NewMockMultiConn(fx.ctrl)
	ctx := CtxWithPeerId(context.Background(), peerId)
	fx.mc.EXPECT().Context().Return(ctx).AnyTimes()
	p, err := NewPeer(fx.mc)
	require.NoError(t, err)
	fx.peer = p.(*peer)
	return fx
}

type fixture struct {
	*peer
	ctrl *gomock.Controller
	mc   *mock_transport.MockMultiConn
}

func (fx *fixture) finish() {
	fx.ctrl.Finish()
}
