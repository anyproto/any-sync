package peer

import (
	"context"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/app/ocache"
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
		active:    map[drpc.Conn]struct{}{},
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

	AcquireDrpcConn(ctx context.Context) (drpc.Conn, error)
	ReleaseDrpcConn(conn drpc.Conn)

	IsClosed() bool

	TryClose(objectTTL time.Duration) (res bool, err error)

	ocache.Object
}

type peer struct {
	id string

	ctrl connCtrl

	// drpc conn pool
	inactive []drpc.Conn
	active   map[drpc.Conn]struct{}

	mu sync.Mutex

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
	if time.Now().Sub(p.LastUsage()) < objectTTL {
		return false, nil
	}
	return true, p.Close()
}

func (p *peer) Close() (err error) {
	log.Debug("peer close", zap.String("peerId", p.id))
	return p.MultiConn.Close()
}
