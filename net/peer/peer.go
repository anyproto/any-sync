package peer

import (
	"github.com/anyproto/any-sync/net/transport"
	"time"

	"github.com/anyproto/any-sync/app/logger"
	"go.uber.org/zap"
)

var log = logger.NewNamed("common.net.peer")

func NewPeer(mc transport.MultiConn) (p Peer, err error) {
	ctx := mc.Context()
	pr := &peer{}
	if pr.id, err = CtxPeerId(ctx); err != nil {
		return
	}
	return pr, nil
}

type Peer interface {
	Id() string
	TryClose(objectTTL time.Duration) (res bool, err error)
	transport.MultiConn
}

type peer struct {
	id string
	transport.MultiConn
}

func (p *peer) Id() string {
	return p.id
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
