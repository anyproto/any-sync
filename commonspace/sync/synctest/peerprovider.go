package synctest

import (
	"sync"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/transport"
)

const PeerName = "peerprovider"

type PeerProvider struct {
	sync.Mutex
	myPeer       string
	peers        map[string]peer.Peer
	connProvider *ConnProvider
	server       *rpctest.TestServer
}

func (c *PeerProvider) Init(a *app.App) (err error) {
	c.connProvider = a.MustComponent(ConnName).(*ConnProvider)
	c.server = a.MustComponent(server.CName).(*rpctest.TestServer)
	c.connProvider.Observe(c, c.myPeer)
	return
}

func (c *PeerProvider) Name() (name string) {
	return PeerName
}

func (c *PeerProvider) StartPeer(peerId string, conn transport.MultiConn) (err error) {
	c.Lock()
	defer c.Unlock()
	c.peers[peerId], err = peer.NewPeer(conn, c.server)
	return err
}

func (c *PeerProvider) GetPeer(peerId string) (pr peer.Peer, err error) {
	c.Lock()
	defer c.Unlock()
	if pr, ok := c.peers[peerId]; ok {
		return pr, nil
	}
	conn := c.connProvider.GetConn(c.myPeer, peerId)
	c.peers[peerId], err = peer.NewPeer(conn, c.server)
	if err != nil {
		return nil, err
	}
	return c.peers[peerId], nil
}

func NewPeerProvider(myPeer string) *PeerProvider {
	return &PeerProvider{myPeer: myPeer, peers: make(map[string]peer.Peer)}
}
