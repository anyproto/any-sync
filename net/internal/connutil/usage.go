package connutil

import (
	"go.uber.org/atomic"
	"net"
	"time"
)

func NewLastUsageConn(conn net.Conn) *LastUsageConn {
	return &LastUsageConn{Conn: conn}
}

type LastUsageConn struct {
	net.Conn
	lastUsage atomic.Time
}

func (c *LastUsageConn) Write(p []byte) (n int, err error) {
	c.lastUsage.Store(time.Now())
	return c.Conn.Write(p)
}

func (c *LastUsageConn) Read(p []byte) (n int, err error) {
	c.lastUsage.Store(time.Now())
	return c.Conn.Read(p)
}

func (c *LastUsageConn) LastUsage() time.Time {
	return c.lastUsage.Load()
}
