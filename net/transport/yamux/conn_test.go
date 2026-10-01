package yamux

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/transport"
)

// newSessionPair returns a client MultiConn whose SYN semaphore holds a single
// slot, and the server session, which acknowledges a SYN only on Accept
func newSessionPair(t *testing.T) (*yamuxConn, *yamux.Session) {
	cc, sc := net.Pipe()
	conf := yamux.DefaultConfig()
	conf.AcceptBacklog = 1
	conf.LogOutput = io.Discard
	client, err := yamux.Client(cc, conf)
	require.NoError(t, err)
	server, err := yamux.Server(sc, conf)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	mc := newMultiConn(context.Background(), connutil.NewLastUsageConn(cc), "pipe", client, time.Second)
	return mc, server
}

func TestYamuxConn_OpenHonoursContext(t *testing.T) {
	mc, _ := newSessionPair(t)

	// the only SYN slot is taken by a stream the server never accepts
	first, err := mc.Open(ctx)
	require.NoError(t, err)
	defer first.Close()

	const attempts = 50
	for i := 0; i < attempts; i++ {
		octx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
		start := time.Now()
		conn, oErr := mc.Open(octx)
		cancel()
		require.Nil(t, conn)
		require.ErrorIs(t, oErr, context.DeadlineExceeded)
		require.Less(t, time.Since(start), time.Second, "Open must return on ctx")
	}
	// the helpers left behind are capped per connection; past the cap Open
	// waits for the backlog within its ctx and leaves no helper
	assert.Equal(t, maxAbandonedOpens, mc.abandoned())

	// a caller waiting on the backlog returns as soon as the session goes
	// away, and the session going away releases every helper
	waitErr := make(chan error, 1)
	go func() {
		lctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		_, oErr := mc.Open(lctx)
		waitErr <- oErr
	}()
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, mc.Session.Close())
	select {
	case oErr := <-waitErr:
		require.Error(t, oErr)
		require.NotErrorIs(t, oErr, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("waiting Open did not return on session close")
	}
	require.Eventually(t, func() bool { return mc.abandoned() == 0 }, 5*time.Second, 10*time.Millisecond)
}

func TestYamuxConn_OpenWaitsForBacklog(t *testing.T) {
	mc, _ := newSessionPair(t)
	// simulate a full backlog of abandoned helpers
	mc.backlogMu.Lock()
	mc.abandonedOpens = maxAbandonedOpens
	mc.backlogMu.Unlock()

	opened := make(chan error, 1)
	go func() {
		lctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		conn, oErr := mc.Open(lctx)
		if oErr == nil {
			_ = conn.Close()
		}
		opened <- oErr
	}()
	select {
	case <-opened:
		t.Fatal("Open must wait while the backlog is full")
	case <-time.After(50 * time.Millisecond):
	}
	// one helper finishes: the waiting caller proceeds and opens
	mc.backlogMu.Lock()
	mc.abandonedOpens--
	close(mc.backlogFreed)
	mc.backlogFreed = make(chan struct{})
	mc.backlogMu.Unlock()
	select {
	case oErr := <-opened:
		require.NoError(t, oErr)
	case <-time.After(5 * time.Second):
		t.Fatal("Open did not proceed once the backlog dropped")
	}
}

func TestYamuxConn_OpensInProgressAreNotThrottled(t *testing.T) {
	mc, _ := newSessionPair(t)

	first, err := mc.Open(ctx)
	require.NoError(t, err)
	defer first.Close()

	// many callers wait on a congested link, none of them gave up
	const waiting = 3 * maxAbandonedOpens
	results := make(chan error, waiting)
	octx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for i := 0; i < waiting; i++ {
		go func() {
			conn, oErr := mc.Open(octx)
			if oErr == nil {
				_ = conn.Close()
			}
			results <- oErr
		}()
	}
	// a caller with a short deadline is not refused because of them
	time.Sleep(50 * time.Millisecond)
	sctx, scancel := context.WithTimeout(ctx, 10*time.Millisecond)
	_, err = mc.Open(sctx)
	scancel()
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// every waiting open returns once the session goes away, and no helper
	// is left behind. (Letting the server catch up instead would hit a yamux
	// quirk: the abandoned helper's stream, closed before the remote accepts
	// it, is never acknowledged and holds the only SYN slot.)
	require.NoError(t, mc.Session.Close())
	for i := 0; i < waiting; i++ {
		select {
		case <-results:
		case <-time.After(5 * time.Second):
			t.Fatal("open did not return")
		}
	}
	require.Eventually(t, func() bool { return mc.abandoned() == 0 }, 5*time.Second, 10*time.Millisecond)
}

func TestYamuxConn_OpenClosesLateStream(t *testing.T) {
	mc, server := newSessionPair(t)

	first, err := mc.Open(ctx)
	require.NoError(t, err)
	defer first.Close()

	octx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	_, err = mc.Open(octx)
	cancel()
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, mc.abandoned())

	// accepting the first stream frees the SYN slot: the helper's stream
	// arrives after its caller left, so the helper closes it
	_, err = server.Accept()
	require.NoError(t, err)
	late, err := server.Accept()
	require.NoError(t, err)
	require.NoError(t, late.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = io.ReadAll(late)
	require.NoError(t, err, "the late stream must be closed by the helper")
	require.Eventually(t, func() bool { return mc.abandoned() == 0 }, 5*time.Second, 10*time.Millisecond)
	// Not asserted: further opens on this session. When the helper's FIN
	// reaches the remote before its Accept, yamux never acknowledges the
	// stream and its SYN slot stays taken until StreamOpenTimeout (a known
	// upstream quirk).
}

func TestYamuxConn_OpenCancelledContext(t *testing.T) {
	mc, _ := newSessionPair(t)
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := mc.Open(cctx)
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, mc.abandoned())
}

func TestYamuxConn_OpenClosedSession(t *testing.T) {
	mc, _ := newSessionPair(t)
	require.NoError(t, mc.Session.Close())
	_, err := mc.Open(ctx)
	require.ErrorIs(t, err, yamux.ErrSessionShutdown)
}

// streamPair opens a stream from mc and accepts it on server
func streamPair(t *testing.T, mc *yamuxConn, server *yamux.Session) (client, remote net.Conn) {
	accepted := make(chan net.Conn, 1)
	go func() {
		s, err := server.Accept()
		if err == nil {
			accepted <- s
		}
	}()
	client, err := mc.Open(ctx)
	require.NoError(t, err)
	// the server sees the stream only once something is written
	_, err = client.Write([]byte("x"))
	require.NoError(t, err)
	select {
	case remote = <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("stream not accepted")
	}
	buf := make([]byte, 1)
	_, err = io.ReadFull(remote, buf)
	require.NoError(t, err)
	return client, remote
}

func readErr(t *testing.T, conn net.Conn) chan error {
	res := make(chan error, 1)
	go func() {
		_, err := conn.Read(make([]byte, 16))
		res <- err
	}()
	return res
}

func waitErr(t *testing.T, ch chan error) error {
	select {
	case err := <-ch:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("read did not return")
		return nil
	}
}

func TestYamuxStream_SessionDeathNormalized(t *testing.T) {
	t.Run("local session close mid-read", func(t *testing.T) {
		mc, server := newSessionPair(t)
		client, _ := streamPair(t, mc, server)
		res := readErr(t, client)
		time.Sleep(20 * time.Millisecond)
		require.NoError(t, mc.Session.Close())
		err := waitErr(t, res)
		assert.ErrorIs(t, err, transport.ErrConnClosed)
		assert.ErrorIs(t, err, io.EOF, "the original error stays reachable")

		_, err = client.Write([]byte("more"))
		assert.ErrorIs(t, err, transport.ErrConnClosed)
		// yamux force-closed the stream on shutdown
		assert.ErrorIs(t, err, yamux.ErrStreamClosed)
	})
	t.Run("remote session close mid-read", func(t *testing.T) {
		mc, server := newSessionPair(t)
		client, _ := streamPair(t, mc, server)
		res := readErr(t, client)
		time.Sleep(20 * time.Millisecond)
		require.NoError(t, server.Close())
		err := waitErr(t, res)
		assert.ErrorIs(t, err, transport.ErrConnClosed)
		assert.True(t, mc.Session.IsClosed())
	})
	t.Run("accepted stream, remote session close", func(t *testing.T) {
		mc, server := newSessionPair(t)
		accepted := make(chan net.Conn, 1)
		go func() {
			if s, err := mc.Accept(); err == nil {
				accepted <- s
			}
		}()
		remote, err := server.Open()
		require.NoError(t, err)
		_, err = remote.Write([]byte("x"))
		require.NoError(t, err)
		var local net.Conn
		select {
		case local = <-accepted:
		case <-time.After(5 * time.Second):
			t.Fatal("stream not accepted")
		}
		_, err = io.ReadFull(local, make([]byte, 1))
		require.NoError(t, err)
		res := readErr(t, local)
		time.Sleep(20 * time.Millisecond)
		require.NoError(t, server.Close())
		assert.ErrorIs(t, waitErr(t, res), transport.ErrConnClosed)
	})
	t.Run("remote stream close is a plain EOF", func(t *testing.T) {
		mc, server := newSessionPair(t)
		client, remote := streamPair(t, mc, server)
		res := readErr(t, client)
		require.NoError(t, remote.Close())
		err := waitErr(t, res)
		assert.Equal(t, io.EOF, err, "a normal stream close must stay io.EOF")
		assert.False(t, mc.Session.IsClosed())
	})
}
