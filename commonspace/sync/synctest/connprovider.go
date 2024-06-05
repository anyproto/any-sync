package synctest

import (
	"sync"

	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/transport"
)

type ConnProvider struct {
	sync.Mutex
	multiConns map[string]transport.MultiConn
}

func (c *ConnProvider) GetConn(firstId, secondId string) (conn transport.MultiConn) {
	c.Lock()
	defer c.Unlock()
	id := mapId(firstId, secondId)
	if conn, ok := c.multiConns[id]; ok {
		return conn
	}
	first, second := rpctest.MultiConnPair(firstId, secondId)
	c.multiConns[id] = second
	c.multiConns[mapId(secondId, firstId)] = first
	return second
}

func NewConnProvider() *ConnProvider {
	return &ConnProvider{
		multiConns: make(map[string]transport.MultiConn),
	}
}

func mapId(firstId, secondId string) string {
	return firstId + "-" + secondId
}
