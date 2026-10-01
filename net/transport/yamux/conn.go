package yamux

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
)

func NewMultiConn(cctx context.Context, luConn *connutil.LastUsageConn, addr string, sess *yamux.Session) transport.MultiConn {
	return newMultiConn(cctx, luConn, addr, sess, 0)
}

func newMultiConn(cctx context.Context, luConn *connutil.LastUsageConn, addr string, sess *yamux.Session, writeTimeout time.Duration) *yamuxConn {
	cctx = peer.CtxWithPeerAddr(cctx, transport.Yamux+"://"+sess.RemoteAddr().String())
	return &yamuxConn{
		ctx:          cctx,
		luConn:       luConn,
		addr:         addr,
		Session:      sess,
		writeTimeout: writeTimeout,
		backlogFreed: make(chan struct{}),
	}
}

// maxAbandonedOpens bounds the Session.Open helpers a single connection
// keeps running for callers that have already given up (see Open)
const maxAbandonedOpens = 16

type yamuxConn struct {
	ctx    context.Context
	luConn *connutil.LastUsageConn
	addr   string
	*yamux.Session
	// writeTimeout is the configured WriteTimeoutSec, which yamux uses as
	// both ConnectionWriteTimeout and StreamCloseTimeout: a stream close is
	// bounded by it, and the peer's cleanup owner derives its stall
	// threshold from it (see WriteTimeout)
	writeTimeout time.Duration

	backlogMu sync.Mutex
	// abandonedOpens counts Open helpers still running after their caller
	// gave up
	abandonedOpens int
	// backlogFreed is closed and replaced whenever abandonedOpens drops
	backlogFreed chan struct{}
}

type openResult struct {
	conn net.Conn
	err  error
}

// Open opens a new stream, bounded by ctx. yamux's Session.Open takes no
// context and blocks while too many SYNs are unacknowledged, which on a
// silent connection lasts until StreamOpenTimeout closes the session. It runs
// in a helper goroutine instead; a caller whose ctx ends leaves the helper
// behind, and the helper closes the stream if it arrives late. Opens in
// progress are not limited (a congested but healthy link must not be
// throttled further), only the abandoned helpers are: while
// maxAbandonedOpens of them are running, Open waits, bounded by ctx, for one
// to finish. The cap is checked before the helper starts, so callers racing
// past it together can overshoot it by their number.
func (y *yamuxConn) Open(ctx context.Context) (conn net.Conn, err error) {
	if ctx.Done() == nil {
		// a context that can never end needs no helper
		return y.Session.Open()
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = y.waitBacklog(ctx); err != nil {
		return nil, err
	}
	var (
		// claimed is taken by whichever side finishes first: the helper
		// (it delivers the result) or the caller giving up (the helper then
		// owns the stream and closes it)
		claimed atomic.Bool
		res     = make(chan openResult, 1)
	)
	go func() {
		stream, sErr := y.Session.Open()
		if claimed.CompareAndSwap(false, true) {
			res <- openResult{conn: stream, err: sErr}
			return
		}
		// the caller is gone: close the late stream. The helper counts as
		// abandoned until the close returns.
		if stream != nil {
			_ = stream.Close()
		}
		y.backlogMu.Lock()
		y.abandonedOpens--
		close(y.backlogFreed)
		y.backlogFreed = make(chan struct{})
		y.backlogMu.Unlock()
	}()
	select {
	case r := <-res:
		return r.conn, r.err
	case <-ctx.Done():
		if claimed.CompareAndSwap(false, true) {
			y.backlogMu.Lock()
			y.abandonedOpens++
			y.backlogMu.Unlock()
			return nil, ctx.Err()
		}
		// the helper won the race and is delivering right now
		r := <-res
		return r.conn, r.err
	}
}

// waitBacklog waits until fewer than maxAbandonedOpens helpers are running
func (y *yamuxConn) waitBacklog(ctx context.Context) error {
	for {
		y.backlogMu.Lock()
		if y.abandonedOpens < maxAbandonedOpens {
			y.backlogMu.Unlock()
			return nil
		}
		freed := y.backlogFreed
		y.backlogMu.Unlock()
		select {
		case <-freed:
		case <-ctx.Done():
			return ctx.Err()
		case <-y.Session.CloseChan():
			return yamux.ErrSessionShutdown
		}
	}
}

// abandoned returns the number of Open helpers left behind by their callers
func (y *yamuxConn) abandoned() int {
	y.backlogMu.Lock()
	defer y.backlogMu.Unlock()
	return y.abandonedOpens
}

// WriteTimeout implements transport.WriteTimeouter
func (y *yamuxConn) WriteTimeout() time.Duration {
	return y.writeTimeout
}

func (y *yamuxConn) LastUsage() time.Time {
	return y.luConn.LastUsage()
}

func (y *yamuxConn) BytesRead() int64 {
	return y.luConn.BytesRead()
}

func (y *yamuxConn) BytesWritten() int64 {
	return y.luConn.BytesWritten()
}

func (y *yamuxConn) Context() context.Context {
	return y.ctx
}

func (y *yamuxConn) Addr() string {
	return transport.Yamux + "://" + y.addr
}

func (y *yamuxConn) Accept() (conn net.Conn, err error) {
	if conn, err = y.Session.Accept(); err != nil {
		if err == yamux.ErrSessionShutdown || err == io.EOF {
			err = transport.ErrConnClosed
		}
		return
	}
	return
}
