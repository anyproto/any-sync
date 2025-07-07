//go:generate mockgen -destination mock_peer/mock_peer.go github.com/anyproto/any-sync/net/peer Peer
package peer

import (
	"context"
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
	pr.acceptCtx, pr.acceptCtxCancel = context.WithCancel(context.Background())
	if pr.id, err = CtxPeerId(ctx); err != nil {
		return
	}
	go pr.acceptLoop()
	return pr, nil
}

type Stat struct {
	PeerId         string    `json:"peerId"`
	SubConnections int       `json:"subConnections"`
	Created        time.Time `json:"created"`
	Version        uint32    `json:"version"`
	AliveTimeSecs  float64   `json:"aliveTimeSecs"`
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

	mu        sync.Mutex
	created   time.Time
	useSnappy bool

	transport.MultiConn
}

func (p *peer) Id() string {
	return p.id
}

func (p *peer) AcquireDrpcConn(ctx context.Context) (drpc.Conn, error) {
	p.mu.Lock()
	if len(p.inactive) == 0 {
		wait := p.limiter.wait(len(p.active) + int(p.openingWaitCount.Load()))
		p.openingWaitCount.Add(1)
		defer p.openingWaitCount.Add(-1)
		p.mu.Unlock()
		if wait != nil {
			// throttle new connection opening
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case dconn := <-p.subConnRelease:
				// nil conn means connection was closed, used to wake up AcquireDrpcConn
				if dconn != nil {
					return dconn, nil
				}
			case <-wait:
			}
		}
		dconn, err := p.openDrpcConn(ctx)
		if err != nil {
			return nil, err
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
		return p.AcquireDrpcConn(ctx)
	default:
	}
	p.active[res] = struct{}{}
	p.mu.Unlock()
	return res, nil
}

// ReleaseDrpcConn releases the connection back to the pool.
// you should pass the same ctx you passed to AcquireDrpcConn
func (p *peer) ReleaseDrpcConn(ctx context.Context, conn drpc.Conn) {
	var closed bool
	select {
	case <-conn.Closed():
		closed = true
	case <-ctx.Done():
		// in case ctx is closed the connection may be not yet closed because of the signal logic in the drpc manager
		// but, we want to shortcut to avoid race conditions
		_ = conn.Close()
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
			default:
				// means the connection has some unfinished work,
				// e.g. not fully read stream
				// we cannot reuse this connection so let's close it
				_ = conn.Close()
				closed = true
			}
		} else {
			panic("conn does not implement Unblocked()")
		}
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
	proto, err := handshake.OutgoingProtoHandshake(ctx, lastUsageConn, defaultHandshakeProto)
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
			if serveErr != io.EOF && serveErr != transport.ErrConnClosed {
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
	if aliveCount == 0 && p.created.Add(time.Minute).Before(time.Now()) {
		return true, p.Close()
	}
	return false, nil
}

func (p *peer) gc(ttl time.Duration) (aliveCount int) {
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
			_ = in.Close()
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
			_ = act.Close()
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
	}
}
