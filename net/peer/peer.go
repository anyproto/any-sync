package peer

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/anytypeio/any-sync/app/logger"
	"github.com/libp2p/go-libp2p/core/sec"
	"go.uber.org/zap"
	"storj.io/drpc"
)

var log = logger.NewNamed("common.net.peer")

func NewPeer(sc sec.SecureConn, conn drpc.Conn) Peer {
	return &peer{
		id:        sc.RemotePeer().String(),
		lastUsage: time.Now().Unix(),
		sc:        sc,
		Conn:      conn,
	}
}

type Peer interface {
	Id() string
	LastUsage() time.Time
	UpdateLastUsage()
	drpc.Conn
}

type peer struct {
	id        string
	lastUsage int64
	sc        sec.SecureConn
	drpc.Conn
}

func (p *peer) Id() string {
	return p.id
}

func (p *peer) LastUsage() time.Time {
	select {
	case <-p.Closed():
		return time.Unix(0, 0)
	default:
	}
	return time.Unix(atomic.LoadInt64(&p.lastUsage), 0)
}

func (p *peer) Invoke(ctx context.Context, rpc string, enc drpc.Encoding, in, out drpc.Message) error {
	defer p.UpdateLastUsage()
	return p.Conn.Invoke(ctx, rpc, enc, in, out)
}

func (p *peer) NewStream(ctx context.Context, rpc string, enc drpc.Encoding) (drpc.Stream, error) {
	defer p.UpdateLastUsage()
	return p.Conn.NewStream(ctx, rpc, enc)
}

func (p *peer) Read(b []byte) (n int, err error) {
	if n, err = p.sc.Read(b); err != nil {
		p.UpdateLastUsage()
	}
	return
}

func (p *peer) Write(b []byte) (n int, err error) {
	if n, err = p.sc.Write(b); err != nil {
		p.UpdateLastUsage()
	}
	return
}

func (p *peer) UpdateLastUsage() {
	atomic.StoreInt64(&p.lastUsage, time.Now().Unix())
}

func (p *peer) Close() (err error) {
	log.Debug("peer close", zap.String("peerId", p.id))
	return p.Conn.Close()
}
