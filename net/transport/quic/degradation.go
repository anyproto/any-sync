package quic

import (
	"context"
	"errors"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/transport"
)

// degradedMaxLifetime separates DPI-degraded connections from ordinary
// idle-timeout deaths. A degraded conn dies within about a minute of
// establishment (the idle timeout, 30s by default, outlasts the configured
// keepalive period), while sleep/network-change deaths
// hit connections that already lived a while. A connection surviving this long
// has proven the path regardless of what finally closed it.
const degradedMaxLifetime = 5 * time.Minute

// classifyClose maps a connection close cause, lifetime and received-byte
// count to a transport.ConnCloseKind.
//
// An idle timeout on a young connection means the path went black under us:
// with keepalives on, a healthy path always has ACK traffic inside the idle
// window. It stays degraded however much data moved first, because the
// censors this detects let a flow run before freezing it.
//
// Anything else that carried data is positive evidence the path works. That
// matters more than it looks: the peer pool closes idle connections after
// about a minute, so peers used for short RPCs never reach
// degradedMaxLifetime, and without this they could never clear their strikes.
func classifyClose(cause error, lifetime time.Duration, bytesRead int64) transport.ConnCloseKind {
	var idle *quic.IdleTimeoutError
	if errors.As(cause, &idle) {
		// An idle timeout always means the path went black: keepalives are on,
		// so a working path never reaches it. Young means the connection was
		// cut shortly after the handshake, which is the signature this
		// detects; old is usually sleep or a network change, which is no
		// evidence either way and must not clear the peer's history.
		if lifetime < degradedMaxLifetime {
			return transport.ConnCloseDegraded
		}
		return transport.ConnCloseNeutral
	}
	if lifetime >= degradedMaxLifetime {
		return transport.ConnCloseHealthy
	}
	var reset *quic.StatelessResetError
	if errors.As(cause, &reset) {
		// the peer answered our packets with a reset token: it lost its state
		// (a restart, or a NAT rebinding), but the path itself demonstrably
		// carries traffic in both directions
		return transport.ConnCloseHealthy
	}
	if bytesRead > 0 {
		return transport.ConnCloseHealthy
	}
	return transport.ConnCloseNeutral
}

// IsDialDegraded reports whether a quic dial error means the path swallowed
// our packets rather than answering them.
//
// Both quic-go timeouts qualify, and the idle one is the case that matters:
// with no packet ever received the idle deadline is measured from connection
// start and fires at HandshakeIdleTimeout, while the handshake deadline is
// twice that — so a blackholed UDP path always reports an idle timeout and
// never a handshake timeout. Caller-context deadlines are excluded: quic-go
// returns context.Cause for those, and an impatient caller says nothing about
// the path.
func IsDialDegraded(err error) bool {
	var ht *quic.HandshakeTimeoutError
	if errors.As(err, &ht) {
		return true
	}
	var idle *quic.IdleTimeoutError
	return errors.As(err, &idle)
}

// watch blocks until the underlying QUIC connection dies, then reports a
// classified close event. Run in a goroutine for every dialed connection when
// an observer is registered.
func (q *quicMultiConn) watch(observer func(ev transport.ConnCloseEvent)) {
	if observer == nil {
		return
	}
	connCtx := q.connection.Context()
	<-connCtx.Done()
	cause := context.Cause(connCtx)
	// wall clock on both sides: see the startTime comment in newConn
	lifetime := time.Now().Round(0).Sub(q.startTime)
	bytesRead := q.bytesRead.Load()
	peerId, _ := peer.CtxPeerId(q.cctx)
	observer(transport.ConnCloseEvent{
		PeerId:       peerId,
		Kind:         classifyClose(cause, lifetime, bytesRead),
		Lifetime:     lifetime,
		BytesRead:    bytesRead,
		BytesWritten: q.bytesWritten.Load(),
		Cause:        cause,
	})
}
