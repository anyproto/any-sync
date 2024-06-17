package synctest

import (
	"context"
	"sync"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/rpc/server"
)

const PeerName = "net.peerservice"

type PeerProvider struct {
	sync.Mutex
	myPeer string
	peers  map[string]peer.Peer
	pool   *PeerGlobalPool
	server *rpctest.TestServer
}

func (c *PeerProvider) Run(ctx context.Context) (err error) {
	c.pool.AddCtrl(c.myPeer, c.server)
	return nil
}

func (c *PeerProvider) Close(ctx context.Context) (err error) {
	return nil
}

func (c *PeerProvider) Init(a *app.App) (err error) {
	c.pool = a.MustComponent(PeerGlobalName).(*PeerGlobalPool)
	c.server = a.MustComponent(server.CName).(*rpctest.TestServer)
	return
}

func (c *PeerProvider) Name() (name string) {
	return PeerName
}

func (c *PeerProvider) GetPeerIds() (peerIds []string) {
	return c.pool.GetPeerIds()
}

func (c *PeerProvider) Dial(ctx context.Context, peerId string) (pr peer.Peer, err error) {
	return c.GetPeer(peerId)
}

func (c *PeerProvider) GetPeer(peerId string) (pr peer.Peer, err error) {
	c.Lock()
	defer c.Unlock()
	if pr, ok := c.peers[peerId]; ok {
		return pr, nil
	}
	c.peers[peerId], err = c.pool.GetPeer(mapId(c.myPeer, peerId))
	if err != nil {
		return nil, err
	}
	return c.peers[peerId], nil
}

func NewPeerProvider(myPeer string) *PeerProvider {
	return &PeerProvider{myPeer: myPeer, peers: make(map[string]peer.Peer)}
}
