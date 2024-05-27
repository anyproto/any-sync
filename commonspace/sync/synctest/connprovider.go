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

func (c *ConnProvider) GetConn(firstId, secondId string) (conn transport.MultiConn) {
	c.Lock()
	defer c.Unlock()
	id := mapId(firstId, secondId)
	if conn, ok := c.multiConns[id]; ok {
		return conn
	}
	first, second := rpctest.MultiConnPair(firstId, secondId)
	c.multiConns[id] = first
	c.multiConns[mapId(secondId, firstId)] = second
	err := c.providers[secondId].StartPeer(secondId, second)
	if err != nil {
		panic(err)
	}
	return first
}

func NewConnProvider() *ConnProvider {
	return &ConnProvider{
		multiConns: make(map[string]transport.MultiConn),
		providers:  make(map[string]*PeerProvider),
	}
}

func mapId(firstId, secondId string) string {
	return firstId + "-" + secondId
}
