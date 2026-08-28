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
// idle-timeout deaths. A degraded conn dies within a minute of establishment
// (keepalive every 25s, idle timeout 30s), while sleep/network-change deaths
// hit connections that already lived a while. A connection surviving this long
// has proven the path regardless of what finally closed it.
const degradedMaxLifetime = 5 * time.Minute

// classifyClose maps a connection close cause and lifetime to a
// transport.ConnCloseKind. An idle timeout on a young connection means the
// path went black under us: with keepalives on, a healthy path always has ACK
// traffic inside the idle window.
func classifyClose(cause error, lifetime time.Duration) transport.ConnCloseKind {
	if lifetime >= degradedMaxLifetime {
		return transport.ConnCloseHealthy
	}
	var idle *quic.IdleTimeoutError
	if errors.As(cause, &idle) {
		return transport.ConnCloseDegraded
	}
	return transport.ConnCloseNeutral
}

// watch blocks until the underlying QUIC connection dies, then reports a
// classified close event. Run in a goroutine for every dialed connection when
// an observer is registered.
func (q *quicMultiConn) watch(observer func(ev transport.ConnCloseEvent)) {
	connCtx := q.connection.Context()
	<-connCtx.Done()
	cause := context.Cause(connCtx)
	lifetime := time.Since(q.startTime)
	peerId, _ := peer.CtxPeerId(q.cctx)
	observer(transport.ConnCloseEvent{
		PeerId:       peerId,
		Kind:         classifyClose(cause, lifetime),
		Lifetime:     lifetime,
		BytesRead:    q.bytesRead.Load(),
		BytesWritten: q.bytesWritten.Load(),
		Cause:        cause,
	})
}
