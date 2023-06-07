//go:generate mockgen -destination mock_peer/mock_peer.go github.com/anyproto/any-sync/net/peer Peer
package peer

import (
	"context"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anyproto/any-sync/net/transport"
	"go.uber.org/zap"
	"io"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"sync"
	"time"
)

var log = logger.NewNamed("common.net.peer")

type connCtrl interface {
	ServeConn(ctx context.Context, conn net.Conn) (err error)
}

func NewPeer(mc transport.MultiConn, ctrl connCtrl) (p Peer, err error) {
	ctx := mc.Context()
	pr := &peer{
		active:    map[*subConn]struct{}{},
		MultiConn: mc,
		ctrl:      ctrl,
	}
	if pr.id, err = CtxPeerId(ctx); err != nil {
		return
	}
	go pr.acceptLoop()
	return pr, nil
}

type Peer interface {
	Id() string
	Context() context.Context

	AcquireDrpcConn(ctx context.Context) (drpc.Conn, error)
	ReleaseDrpcConn(conn drpc.Conn)
	DoDrpc(ctx context.Context, do func(conn drpc.Conn) error) error

	IsClosed() bool

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
	inactive []*subConn
	active   map[*subConn]struct{}

	mu sync.Mutex

	transport.MultiConn
}

func (p *peer) Id() string {
	return p.id
}

func (p *peer) AcquireDrpcConn(ctx context.Context) (drpc.Conn, error) {
	p.mu.Lock()
	if len(p.inactive) == 0 {
		p.mu.Unlock()
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
	p.active[res] = struct{}{}
	p.mu.Unlock()
	return res, nil
}

func (p *peer) ReleaseDrpcConn(conn drpc.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	sc, ok := conn.(*subConn)
	if !ok {
		return
	}
	if _, ok = p.active[sc]; ok {
		delete(p.active, sc)
	}
	p.inactive = append(p.inactive, sc)
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

func (p *peer) openDrpcConn(ctx context.Context) (dconn *subConn, err error) {
	conn, err := p.Open(ctx)
	if err != nil {
		return nil, err
	}
	if err = handshake.OutgoingProtoHandshake(ctx, conn, handshakeproto.ProtoType_DRPC); err != nil {
		return nil, err
	}
	tconn := connutil.NewLastUsageConn(conn)
	return &subConn{
		Conn:          drpcconn.New(tconn),
		LastUsageConn: tconn,
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
		conn, err := p.Accept()
		if err != nil {
			exitErr = err
			return
		}
		go func() {
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
}

func (p *peer) serve(conn net.Conn) (err error) {
	hsCtx, cancel := context.WithTimeout(p.Context(), time.Second*20)
	if _, err = handshake.IncomingProtoHandshake(hsCtx, conn, defaultProtoChecker); err != nil {
		cancel()
		return
	}
	cancel()
	return p.ctrl.ServeConn(p.Context(), conn)
}

func (p *peer) TryClose(objectTTL time.Duration) (res bool, err error) {
	p.gc(objectTTL)
	if time.Now().Sub(p.LastUsage()) < objectTTL {
		return false, nil
	}
	return true, p.Close()
}

func (p *peer) gc(ttl time.Duration) {
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
		default:
		}
	}
}

func (p *peer) Close() (err error) {
	log.Debug("peer close", zap.String("peerId", p.id))
	return p.MultiConn.Close()
}
