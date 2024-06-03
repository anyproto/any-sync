package synctest

import (
	"sync"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/transport"
)

const ConnName = "connprovider"

type ConnProvider struct {
	sync.Mutex
	multiConns map[string]transport.MultiConn
	providers  map[string]*PeerProvider
	peerIds    []string
}

func (c *ConnProvider) Init(a *app.App) (err error) {
	return
}

func (c *ConnProvider) Name() (name string) {
	return ConnName
}

func (c *ConnProvider) Observe(provider *PeerProvider, peerId string) {
	c.Lock()
	defer c.Unlock()
	c.providers[peerId] = provider
}

func (c *ConnProvider) GetPeerIds() []string {
	return c.peerIds
}

func (c *ConnProvider) GetConn(firstId, secondId string) (conn transport.MultiConn) {
	c.Lock()
	defer c.Unlock()
	if firstId == secondId {
		panic("cannot connect to self")
	}
	id := mapId(firstId, secondId)
	if conn, ok := c.multiConns[id]; ok {
		return conn
	}
	first, second := rpctest.MultiConnPair(firstId, secondId)
	c.multiConns[id] = second
	c.multiConns[mapId(secondId, firstId)] = first
	err := c.providers[secondId].StartPeer(secondId, second)
	if err != nil {
		panic(err)
	}
	return second
}

func NewConnProvider(peerIds []string) *ConnProvider {
	return &ConnProvider{
		peerIds:    peerIds,
		multiConns: make(map[string]transport.MultiConn),
		providers:  make(map[string]*PeerProvider),
	}
}

func mapId(firstId, secondId string) string {
	return firstId + "-" + secondId
}
