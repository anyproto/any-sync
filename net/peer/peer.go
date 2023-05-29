package peer

import (
	"context"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/net/transport"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"sync"
	"time"

	"github.com/anyproto/any-sync/app/logger"
	"go.uber.org/zap"
)

var log = logger.NewNamed("common.net.peer")

func NewPeer(mc transport.MultiConn) (p Peer, err error) {
	ctx := mc.Context()
	pr := &peer{
		active:    map[drpc.Conn]struct{}{},
		MultiConn: mc,
	}
	if pr.id, err = CtxPeerId(ctx); err != nil {
		return
	}
	return pr, nil
}

type Peer interface {
	Id() string

	AcquireDrpcConn(ctx context.Context) (drpc.Conn, error)
	ReleaseDrpcConn(conn drpc.Conn)

	IsClosed() bool

	TryClose(objectTTL time.Duration) (res bool, err error)

	ocache.Object
}

type peer struct {
	id string

	// drpc conn pool
	inactive []drpc.Conn
	active   map[drpc.Conn]struct{}
	mu       sync.Mutex

	transport.MultiConn
}

func (p *peer) Id() string {
	return p.id
}

func (p *peer) AcquireDrpcConn(ctx context.Context) (drpc.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.inactive) == 0 {
		conn, err := p.Open(ctx)
		if err != nil {
			return nil, err
		}
		dconn := drpcconn.New(conn)
		p.inactive = append(p.inactive, dconn)
	}
	idx := len(p.inactive) - 1
	res := p.inactive[idx]
	p.inactive = p.inactive[:idx]
	p.active[res] = struct{}{}
	return res, nil
}

func (p *peer) ReleaseDrpcConn(conn drpc.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.active[conn]; ok {
		delete(p.active, conn)
	}
	p.inactive = append(p.inactive, conn)
	return
}

func (p *peer) TryClose(objectTTL time.Duration) (res bool, err error) {
	if time.Now().Sub(p.LastUsage()) < objectTTL {
		return false, nil
	}
	p.mu.Lock()
	if len(p.active) > 0 {
		p.mu.Unlock()
		return false, nil
	}
	p.mu.Unlock()
	return true, p.Close()
}

func (p *peer) Close() (err error) {
	log.Debug("peer close", zap.String("peerId", p.id))
	return p.MultiConn.Close()
}
