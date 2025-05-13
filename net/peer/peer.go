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
	ReleaseDrpcConn(conn drpc.Conn)
	DoDrpc(ctx context.Context, do func(conn drpc.Conn) error) error

	IsClosed() bool
	CloseChan() <-chan struct{}

	// SetTTL overrides the default pool ttl
	SetTTL(ttl time.Duration)

	TryClose(objectTTL time.Duration) (res bool, err error)

	ocache.Object
}

type subConn struct {
	drpc.Conn
	*connutil.LastUsageConn
}

type peer struct {
	id string

	ctrl connCtrl

	// drpc conn pool
	// outgoing
	inactive         []*subConn
	active           map[*subConn]struct{}
	subConnRelease   chan drpc.Conn
	openingWaitCount atomic.Int32

	incomingCount atomic.Int32
	acceptCtx     context.Context

	acceptCtxCancel context.CancelFunc

	ttl atomic.Uint32

	limiter limiter

	mu      sync.Mutex
	created time.Time
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
				return dconn, nil
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

func (p *peer) ReleaseDrpcConn(conn drpc.Conn) {
	var closed bool
	select {
	case <-conn.Closed():
		closed = true
	default:
	}

	// try to send this connection to acquire if anyone is waiting for it
	select {
	case p.subConnRelease <- conn:
		return
	default:
	}

	// return to pool
	p.mu.Lock()
	defer p.mu.Unlock()
	sc, ok := conn.(*subConn)
	if !ok {
		return
	}
	if _, ok = p.active[sc]; ok {
		delete(p.active, sc)
	}
	if !closed {
		p.inactive = append(p.inactive, sc)
	}
	return
}

func (p *peer) DoDrpc(ctx context.Context, do func(conn drpc.Conn) error) error {
	conn, err := p.AcquireDrpcConn(ctx)
	if err != nil {
		return err
	}
	defer p.ReleaseDrpcConn(conn)
	return do(conn)
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
		Conn:          encoding.WrapConnEncoding(drpcConn, isSnappy),
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

func (p *peer) serve(conn net.Conn) (err error) {
	defer func() {
		_ = conn.Close()
	}()
	hsCtx, cancel := context.WithTimeout(p.Context(), time.Second*20)
	proto, err := handshake.IncomingProtoHandshake(hsCtx, conn, defaultProtoChecker)
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
