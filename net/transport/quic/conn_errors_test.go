package quic

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/net/transport"
)

func TestWrapConnDead(t *testing.T) {
	dead := []struct {
		name string
		err  error
	}{
		{"idle timeout", &quic.IdleTimeoutError{}},
		{"stateless reset", &quic.StatelessResetError{}},
		{"handshake timeout", &quic.HandshakeTimeoutError{}},
		{"transport error", &quic.TransportError{ErrorCode: quic.InternalError, Remote: true}},
		{"application close code 2", &quic.ApplicationError{ErrorCode: 2, Remote: true}},
		{"wrapped idle timeout", fmt.Errorf("read: %w", &quic.IdleTimeoutError{})},
		{"net.ErrClosed", net.ErrClosed},
	}
	for _, tc := range dead {
		t.Run(tc.name, func(t *testing.T) {
			err := wrapConnDead(tc.err)
			assert.True(t, errors.Is(err, transport.ErrConnClosed), "must match ErrConnClosed")
			assert.True(t, errors.Is(err, net.ErrClosed), "must keep matching net.ErrClosed")
			// the original error stays reachable
			assert.True(t, errors.Is(err, tc.err) || errors.Is(err, errors.Unwrap(tc.err)))
			assert.Contains(t, err.Error(), tc.err.Error())
			// wrapping is idempotent
			assert.Equal(t, err, wrapConnDead(err))
		})
	}
	t.Run("errors.As finds the quic error", func(t *testing.T) {
		err := wrapConnDead(&quic.StatelessResetError{})
		var reset *quic.StatelessResetError
		assert.True(t, errors.As(err, &reset))

		err = wrapConnDead(&quic.ApplicationError{ErrorCode: 2, Remote: true})
		var appErr *quic.ApplicationError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, quic.ApplicationErrorCode(2), appErr.ErrorCode)

		err = wrapConnDead(&quic.TransportError{ErrorCode: quic.ProtocolViolation})
		var trErr *quic.TransportError
		require.True(t, errors.As(err, &trErr))
		assert.Equal(t, quic.ProtocolViolation, trErr.ErrorCode)
	})

	notDead := []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"eof", io.EOF},
		{"stream reset", &quic.StreamError{StreamID: 4, ErrorCode: 0, Remote: true}},
		{"local stream cancel", &quic.StreamError{StreamID: 4, ErrorCode: 0}},
		{"write deadline", os.ErrDeadlineExceeded},
		{"other", errors.New("other")},
	}
	for _, tc := range notDead {
		t.Run("not dead: "+tc.name, func(t *testing.T) {
			err := wrapConnDead(tc.err)
			assert.Equal(t, tc.err, err, "must be returned unchanged")
			if err != nil {
				assert.False(t, errors.Is(err, transport.ErrConnClosed))
			}
		})
	}
}

func TestQuicNetConn_ConnCloseNormalized(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC := dialRetry(t, fxC, fxS.addr)
	var mcS transport.MultiConn
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(time.Second * 5):
		t.Fatal("timeout")
	}

	conn, err := mcC.Open(ctx)
	require.NoError(t, err)
	_, err = conn.Write([]byte("hello"))
	require.NoError(t, err)
	sConn, err := mcS.Accept()
	require.NoError(t, err)
	buf := make([]byte, 5)
	_, err = io.ReadFull(sConn, buf)
	require.NoError(t, err)

	// the server closes the whole connection (application code 2)
	require.NoError(t, mcS.Close())
	select {
	case <-mcC.CloseChan():
	case <-time.After(5 * time.Second):
		t.Fatal("client did not observe the close")
	}

	_, err = conn.Read(buf)
	require.Error(t, err)
	assert.ErrorIs(t, err, transport.ErrConnClosed)
	var appErr *quic.ApplicationError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, quic.ApplicationErrorCode(2), appErr.ErrorCode)

	_, err = conn.Write([]byte("again"))
	require.Error(t, err)
	assert.ErrorIs(t, err, transport.ErrConnClosed)
	assert.ErrorAs(t, err, &appErr)
}

func TestQuicNetConn_StreamResetNotNormalized(t *testing.T) {
	fxS := newFixture(t)
	defer fxS.finish(t)
	fxC := newFixture(t)
	defer fxC.finish(t)

	mcC := dialRetry(t, fxC, fxS.addr)
	var mcS transport.MultiConn
	select {
	case mcS = <-fxS.accepter.mcs:
	case <-time.After(30 * time.Second):
		t.Fatal("timeout")
	}

	conn, err := mcC.Open(ctx)
	require.NoError(t, err)
	_, err = conn.Write([]byte("hello"))
	require.NoError(t, err)
	sConn, err := mcS.Accept()
	require.NoError(t, err)
	buf := make([]byte, 5)
	_, err = io.ReadFull(sConn, buf)
	require.NoError(t, err)

	// only the stream goes away: the server cancels its read side, which
	// makes the client's writes fail with a stream error
	sConn.(quicNetConn).CancelRead(42)
	// the STOP_SENDING takes a round trip: keep writing until it lands
	writeErr := make(chan error, 1)
	go func() {
		for {
			if _, wErr := conn.Write([]byte("more")); wErr != nil {
				writeErr <- wErr
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	select {
	case err = <-writeErr:
	case <-time.After(30 * time.Second):
		t.Fatal("the stream reset never reached the writer")
	}
	var streamErr *quic.StreamError
	require.ErrorAs(t, err, &streamErr)
	assert.False(t, errors.Is(err, transport.ErrConnClosed))
	assert.False(t, mcC.IsClosed())
}

// dialRetry dials, retrying a dial that timed out: under a loaded -race run a
// loopback QUIC handshake can idle out, which is not what these tests are about
func dialRetry(t *testing.T, fx *fixture, addr string) transport.MultiConn {
	var err error
	for i := 0; i < 3; i++ {
		var mc transport.MultiConn
		if mc, err = fx.Dial(ctx, addr); err == nil {
			return mc
		}
	}
	require.NoError(t, err)
	return nil
}
