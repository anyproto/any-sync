//go:generate mockgen -destination mock_peer/mock_peer.go github.com/anyproto/any-sync/net/peer Peer
package peer

import (
	"context"
	"errors"
	"io"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcstream"
	"storj.io/drpc/drpcwire"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/rpc"
	"github.com/anyproto/any-sync/net/rpc/encoding"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anyproto/any-sync/net/transport"
)

var log = logger.NewNamed("common.net.peer")

type connCtrl interface {
	ServeConn(ctx context.Context, conn net.Conn) (err error)
	DrpcConfig() rpc.Config
}

func NewPeer(mc transport.MultiConn, ctrl connCtrl) (p Peer, err error) {
	ctx := mc.Context()
	pr := &peer{
		active:    map[*subConn]struct{}{},
		MultiConn: mc,
		ctrl:      ctrl,
		limiter: limiter{
			// start throttling after 10 sub conns
			startThreshold: 10,
			slowDownStep:   time.Millisecond * 100,
		},
		subConnRelease: make(chan drpc.Conn),
		created:        time.Now(),
		useSnappy:      ctrl.DrpcConfig().Snappy,
	}
	if ttl := CtxTTL(ctx); ttl > 0 {
		pr.SetTTL(ttl)
	}
	pr.acceptCtx, pr.acceptCtxCancel = context.WithCancel(context.Background())
	if pr.id, err = CtxPeerId(ctx); err != nil {
		return
	}
	pr.cleanup = newCleanupOwner(pr.id)
	go pr.acceptLoop()
	return pr, nil
}

type Stat struct {
	PeerId         string    `json:"peerId"`
	SubConnections int       `json:"subConnections"`
	Created        time.Time `json:"created"`
	Version        uint32    `json:"version"`
	AliveTimeSecs  float64   `json:"aliveTimeSecs"`
	BytesRead      int64     `json:"bytesRead"`
	BytesWritten   int64     `json:"bytesWritten"`
}

type StatProvider interface {
	ProvideStat() *Stat
}

type Peer interface {
	Id() string
	Context() context.Context

	AcquireDrpcConn(ctx context.Context) (drpc.Conn, error)
	ReleaseDrpcConn(ctx context.Context, conn drpc.Conn)
	DoDrpc(ctx context.Context, do func(conn drpc.Conn) error) error

	IsClosed() bool
	CloseChan() <-chan struct{}

	// SetTTL overrides the default pool ttl
	SetTTL(ttl time.Duration)

	TryClose(objectTTL time.Duration) (res bool, err error)

	ocache.Object
}

type subConn struct {
	encoding.ConnUnblocked
	*connutil.LastUsageConn
	// doomed is set by gc when it takes an active conn away and hands its
	// close to the cleanup owner: the holder must not return it for reuse,
	// even though the close may not have landed yet
	doomed atomic.Bool
}

func (s *subConn) Unblocked() <-chan struct{} {
	return s.ConnUnblocked.Unblocked()
}

type peer struct {
	id string

	ctrl connCtrl

	// drpc conn pool
	// outgoing
	inactive         []*subConn
	active           map[*subConn]struct{}
	subConnRelease   chan drpc.Conn // can send nil
	openingWaitCount atomic.Int32

	incomingCount atomic.Int32
	acceptCtx     context.Context

	acceptCtxCancel context.CancelFunc

	ttl atomic.Uint32

	limiter limiter

	// cleanup closes sub connections off the callers' path
	cleanup *cleanupOwner

	mu        sync.Mutex
	created   time.Time
	useSnappy bool

	transport.MultiConn
}

func (p *peer) Id() string {
	return p.id
}

func (p *peer) AcquireDrpcConn(ctx context.Context) (drpc.Conn, error) {
	for {
		conn, retry, err := p.acquireDrpcConn(ctx)
		if !retry {
			return conn, err
		}
	}
}

// acquireDrpcConn makes one acquisition attempt; retry means start over
// with a fresh look at the pool and a fresh limiter wait
func (p *peer) acquireDrpcConn(ctx context.Context) (conn drpc.Conn, retry bool, err error) {
	if p.IsClosed() {
		return nil, false, transport.ErrConnClosed
	}
	p.mu.Lock()
	if len(p.inactive) == 0 {
		// closes still in progress count too: they used to run on the
		// releasing callers' goroutines while the conn was still active,
		// and the owner must not turn into a way around the throttling
		wait := p.limiter.wait(len(p.active) + int(p.openingWaitCount.Load()) + p.cleanup.inFlight())
		p.openingWaitCount.Add(1)
		defer p.openingWaitCount.Add(-1)
		p.mu.Unlock()
		if wait != nil {
			// throttle new connection opening
			select {
			case <-ctx.Done():
				return nil, false, ctx.Err()
			case dconn := <-p.subConnRelease:
				// nil conn means connection was closed, used to wake up AcquireDrpcConn
				if dconn != nil && !isDoomed(dconn) {
					return dconn, false, nil
				}
				// The released conn was closed, or gc doomed it on the way.
				// Its close may still be in flight in the cleanup owner, so
				// opening right away would bypass the throttling: start
				// over, which picks up an inactive conn or recomputes the
				// wait with the closes still in flight.
				return nil, true, nil
			case <-wait:
			}
		}
		dconn, err := p.openDrpcConn(ctx)
		if err != nil {
			return nil, false, err
		}
		p.mu.Lock()
		p.inactive = append(p.inactive, dconn)
	}
	idx := len(p.inactive) - 1
	res := p.inactive[idx]
	p.inactive = p.inactive[:idx]
	select {
	case <-res.Closed():
		p.mu.Unlock()
		return nil, true, nil
	default:
	}
	if res.doomed.Load() {
		// gc took it while it was being re-pooled; its close is pending
		p.mu.Unlock()
		return nil, true, nil
	}
	p.active[res] = struct{}{}
	p.mu.Unlock()
	return res, false, nil
}

func isDoomed(conn drpc.Conn) bool {
	sc, ok := conn.(*subConn)
	return ok && sc.doomed.Load()
}

// ReleaseDrpcConn releases the connection back to the pool.
// you should pass the same ctx you passed to AcquireDrpcConn
func (p *peer) ReleaseDrpcConn(ctx context.Context, conn drpc.Conn) {
	var closed bool
	if isDoomed(conn) {
		// gc has taken it out of active and owns its close
		closed = true
	} else {
		closed = p.checkReleased(ctx, conn)
	}

	if !closed {
		select {
		case p.subConnRelease <- conn:
			// shortcut to send a reusable connection
			return
		default:
		}
	}

	sc, ok := conn.(*subConn)
	if !ok {
		return
	}

	p.mu.Lock()

	if _, ok = p.active[sc]; ok {
		delete(p.active, sc)
	}

	if !closed && sc.doomed.Load() {
		// gc doomed it after the check above; doomed is set under p.mu, so
		// this re-check is final
		closed = true
	}
	if !closed {
		// put it back into the pool
		p.inactive = append(p.inactive, sc)
	}
	p.mu.Unlock()

	if closed {
		select {
		case p.subConnRelease <- nil:
			// wake up the waiting AcquireDrpcConn
			// it will take the next one from the inactive pool
			return
		default:
		}
	}
}

// checkReleased reports whether a released conn is closed or must not be
// reused; a conn that must not is closed in the background
func (p *peer) checkReleased(ctx context.Context, conn drpc.Conn) (closed bool) {
	select {
	case <-conn.Closed():
		closed = true
	case <-ctx.Done():
		// in case ctx is closed the connection may be not yet closed because of the signal logic in the drpc manager
		// but, we want to shortcut to avoid race conditions: the conn is never reused, and it is closed in the
		// background, since a drpc close waits for its reader and the transport and the caller is past its deadline
		select {
		case <-conn.Closed():
			// both were ready: nothing left to close
		default:
			p.cleanup.close(conn)
		}
		closed = true
	default:
		if connCasted, ok := conn.(encoding.ConnUnblocked); ok {
			select {
			case <-conn.Closed():
				closed = true
			case <-connCasted.Unblocked():
				// semi-safe to reuse this connection
				// it may be still a chance that connection will be closed in next milliseconds
				// but this is a trade-off for performance
			case <-time.After(time.Second / 5):
				// means the connection has some unfinished work,
				// e.g. not fully read stream
				// we cannot reuse this connection so let's close it
				p.cleanup.close(conn)
				closed = true
			}
		} else {
			// By construction, conns returned from AcquireDrpcConn are *subConn
			// which embeds encoding.ConnUnblocked. Reaching this branch means
			// the caller passed a foreign conn; close it defensively instead
			// of crashing the process.
			log.Warn("released conn does not implement encoding.ConnUnblocked, closing", zap.String("peerId", p.id))
			p.cleanup.close(conn)
			closed = true
		}
	}
	return closed
}

func (p *peer) DoDrpc(ctx context.Context, do func(conn drpc.Conn) error) error {
	conn, err := p.AcquireDrpcConn(ctx)
	if err != nil {
		log.Debug("DoDrpc failed to acquire connection", zap.String("peerId", p.id), zap.Error(err))
		return err
	}
	err = do(conn)
	defer p.ReleaseDrpcConn(ctx, conn)
	return err
}

var defaultHandshakeProto = &handshakeproto.Proto{
	Proto:     handshakeproto.ProtoType_DRPC,
	Encodings: []handshakeproto.Encoding{handshakeproto.Encoding_Snappy, handshakeproto.Encoding_None},
}

func (p *peer) openDrpcConn(ctx context.Context) (*subConn, error) {
	conn, err := p.Open(ctx)
	if err != nil {
		return nil, err
	}
	lastUsageConn := connutil.NewLastUsageConn(conn)
	// on any error the handshake hands the stream to the cleanup owner, once:
	// on a stalled transport the close blocks, so it never runs here
	proto, err := handshake.OutgoingProtoHandshakeWithCloser(ctx, lastUsageConn, defaultHandshakeProto, p.closeSubConn)
	if err != nil {
		return nil, err
	}
	bufSize := p.ctrl.DrpcConfig().Stream.MaxMsgSizeMb * (1 << 20)
	drpcConn := drpcconn.NewWithOptions(lastUsageConn, drpcconn.Options{
		Manager: drpcmanager.Options{
			Reader: drpcwire.ReaderOptions{MaximumBufferSize: bufSize},
			Stream: drpcstream.Options{MaximumBufferSize: bufSize},
		},
	})
	isSnappy := slices.Contains(proto.Encodings, handshakeproto.Encoding_Snappy)
	return &subConn{
		ConnUnblocked: encoding.WrapConnEncoding(drpcConn, isSnappy),
		LastUsageConn: lastUsageConn,
	}, nil
}

func (p *peer) closeSubConn(conn net.Conn) {
	p.cleanup.close(conn)
}

func (p *peer) acceptLoop() {
	var exitErr error
	defer func() {
		if exitErr != transport.ErrConnClosed {
			log.Warn("accept error: close connection", zap.Error(exitErr))
			_ = p.MultiConn.Close()
		}
	}()
	for {
		if wait := p.limiter.wait(int(p.incomingCount.Load())); wait != nil {
			select {
			case <-wait:
			case <-p.acceptCtx.Done():
				return
			}
		}
		conn, err := p.Accept()
		if err != nil {
			exitErr = err
			return
		}
		go func() {
			p.incomingCount.Add(1)
			defer p.incomingCount.Add(-1)
			serveErr := p.serve(conn)
			if serveErr != io.EOF && !errors.Is(serveErr, transport.ErrConnClosed) {
				log.InfoCtx(p.Context(), "serve connection error", zap.Error(serveErr))
			}
		}()
	}
}

var defaultProtoChecker = handshake.ProtoChecker{
	AllowedProtoTypes: []handshakeproto.ProtoType{
		handshakeproto.ProtoType_DRPC,
	},
	SupportedEncodings: []handshakeproto.Encoding{handshakeproto.Encoding_Snappy, handshakeproto.Encoding_None},
}

var noSnappyProtoChecker = handshake.ProtoChecker{
	AllowedProtoTypes: []handshakeproto.ProtoType{
		handshakeproto.ProtoType_DRPC,
	},
}

func (p *peer) serve(conn net.Conn) (err error) {
	defer func() {
		_ = conn.Close()
	}()
	hsCtx, cancel := context.WithTimeout(p.Context(), time.Second*20)
	protoChecker := defaultProtoChecker
	if !p.useSnappy {
		protoChecker = noSnappyProtoChecker
	}
	proto, err := handshake.IncomingProtoHandshake(hsCtx, conn, protoChecker)
	if err != nil {
		cancel()
		return
	}
	cancel()
	ctx := p.Context()
	if slices.Contains(proto.Encodings, handshakeproto.Encoding_Snappy) {
		ctx = encoding.CtxWithSnappy(ctx)
	}
	return p.ctrl.ServeConn(ctx, conn)
}

func (p *peer) SetTTL(ttl time.Duration) {
	p.ttl.Store(uint32(ttl.Seconds()))
}

func (p *peer) TryClose(objectTTL time.Duration) (res bool, err error) {
	if ttl := p.ttl.Load(); ttl > 0 {
		objectTTL = time.Duration(ttl) * time.Second
	}
	aliveCount := p.gc(objectTTL)
	log.Debug("peer gc", zap.String("peerId", p.id), zap.Int("aliveCount", aliveCount))
	// a peer without sub conns is closed once it is older than its TTL (at
	// least a minute): a connection held for reachability alone, with a long
	// TTL, must not be dropped every GC pass
	if aliveCount == 0 && p.created.Add(max(objectTTL, time.Minute)).Before(time.Now()) {
		return true, p.Close()
	}
	return false, nil
}

func (p *peer) gc(ttl time.Duration) (aliveCount int) {
	// drpc conn Close blocks until its reader unwinds, which on a stalled stream
	// takes until the yamux stream close timeout: collect the doomed conns and
	// hand them to the cleanup owner after releasing the lock, so a stalled
	// peer does not hold up the GC pass of every other peer
	var toClose []*subConn
	defer func() {
		for _, conn := range toClose {
			p.cleanup.close(conn)
		}
	}()
	p.mu.Lock()
	defer p.mu.Unlock()
	minLastUsage := time.Now().Add(-ttl)
	var hasClosed bool
	for i, in := range p.inactive {
		select {
		case <-in.Closed():
			p.inactive[i] = nil
			hasClosed = true
		default:
		}
		if in.LastUsage().Before(minLastUsage) {
			toClose = append(toClose, in)
			p.inactive[i] = nil
			hasClosed = true
		}
	}
	if hasClosed {
		inactive := p.inactive
		p.inactive = p.inactive[:0]
		for _, in := range inactive {
			if in != nil {
				p.inactive = append(p.inactive, in)
			}
		}
	}
	for act := range p.active {
		select {
		case <-act.Closed():
			delete(p.active, act)
			continue
		default:
		}
		if act.LastUsage().Before(minLastUsage) {
			log.Warn("close active connection because no activity", zap.String("peerId", p.id), zap.String("addr", p.Addr()))
			act.doomed.Store(true)
			toClose = append(toClose, act)
			delete(p.active, act)
			continue
		}
	}
	return len(p.active) + len(p.inactive) + int(p.incomingCount.Load())
}

func (p *peer) Close() (err error) {
	log.Debug("peer close", zap.String("peerId", p.id))
	return p.MultiConn.Close()
}

func (p *peer) ProvideStat() *Stat {
	p.mu.Lock()
	defer p.mu.Unlock()
	protoVersion, _ := CtxProtoVersion(p.Context())
	subConnectionsCount := len(p.active)
	return &Stat{
		PeerId:         p.id,
		SubConnections: subConnectionsCount,
		Created:        p.created,
		Version:        protoVersion,
		AliveTimeSecs:  time.Now().Sub(p.created).Seconds(),
		BytesRead:      p.MultiConn.BytesRead(),
		BytesWritten:   p.MultiConn.BytesWritten(),
	}
}
