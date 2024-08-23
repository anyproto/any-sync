package synctest

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc"
)

const PeerGlobalName = "peerglobalpool"

type connCtrl interface {
	ServeConn(ctx context.Context, conn net.Conn) (err error)
	DrpcConfig() rpc.Config
}

type connCtrlWrapper struct {
	connCtrl
	setChan chan struct{}
}

func (c *connCtrlWrapper) ServeConn(ctx context.Context, conn net.Conn) (err error) {
	<-c.setChan
	return c.connCtrl.ServeConn(ctx, conn)
}

func (c *connCtrlWrapper) DrpcConfig() rpc.Config {
	<-c.setChan
	return c.connCtrl.DrpcConfig()
}

type PeerGlobalPool struct {
	ctrls        map[string]*connCtrlWrapper
	peers        map[string]peer.Peer
	peerIds      []string
	connProvider *ConnProvider
	sync.Mutex
}

func NewPeerGlobalPool(peerIds []string) *PeerGlobalPool {
	return &PeerGlobalPool{
		peerIds:      peerIds,
		ctrls:        make(map[string]*connCtrlWrapper),
		peers:        make(map[string]peer.Peer),
		connProvider: NewConnProvider(),
	}
}

func (p *PeerGlobalPool) Init(a *app.App) (err error) {
	return nil
}

func (p *PeerGlobalPool) Name() (name string) {
	return PeerGlobalName
}

func (p *PeerGlobalPool) MakePeers() {
	p.Lock()
	defer p.Unlock()
	for _, first := range p.peerIds {
		for _, second := range p.peerIds {
			if first == second {
				continue
			}
			id := mapId(first, second)
			p.ctrls[id] = &connCtrlWrapper{
				setChan: make(chan struct{}),
			}
			conn := p.connProvider.GetConn(first, second)
			p.peers[id], _ = peer.NewPeer(conn, p.ctrls[id])
		}
	}
}

func (p *PeerGlobalPool) GetPeerIds() (peerIds []string) {
	return p.peerIds
}

func (p *PeerGlobalPool) AddCtrl(peerId string, addCtrl connCtrl) {
	p.Lock()
	defer p.Unlock()
	for id, ctrl := range p.ctrls {
		splitId := strings.Split(id, "-")
		if splitId[0] == peerId {
			ctrl.connCtrl = addCtrl
			close(ctrl.setChan)
		}
	}
}

func (p *PeerGlobalPool) GetPeer(id string) (peer.Peer, error) {
	p.Lock()
	defer p.Unlock()
	if pr, ok := p.peers[id]; ok {
		return pr, nil
	}
	return nil, fmt.Errorf("peer not found")
}
