package quic

import (
	"context"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/quic/mock_quic"
)

func TestQuicMultiConn_Open_IdleTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mock_quic.NewMockconnection(ctrl)
	q := &quicMultiConn{
		cctx:         context.Background(),
		connection:   mockConn,
		writeTimeout: time.Second,
		closeTimeout: 100 * time.Millisecond,
	}
	mockConn.EXPECT().OpenStreamSync(gomock.Any()).Return(nil, &quic.IdleTimeoutError{})

	_, err := q.Open(context.Background())
	assert.ErrorIs(t, err, transport.ErrConnClosed)
}

func TestQuicMultiConn_Accept_IdleTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mock_quic.NewMockconnection(ctrl)
	q := &quicMultiConn{
		cctx:         context.Background(),
		connection:   mockConn,
		writeTimeout: time.Second,
		closeTimeout: 100 * time.Millisecond,
	}
	mockConn.EXPECT().AcceptStream(gomock.Any()).Return(nil, &quic.IdleTimeoutError{})

	_, err := q.Accept()
	assert.ErrorIs(t, err, transport.ErrConnClosed)
}
