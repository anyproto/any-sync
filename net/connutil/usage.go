package connutil

import (
	"net"
	"sync/atomic"
	"time"
)

func NewLastUsageConn(conn net.Conn) *LastUsageConn {
	return &LastUsageConn{Conn: conn}
}

type LastUsageConn struct {
	net.Conn
	lastUsageUnixNano atomic.Int64
	bytesRead         atomic.Int64
	bytesWritten      atomic.Int64
}

func (c *LastUsageConn) Write(p []byte) (n int, err error) {
	c.lastUsageUnixNano.Store(time.Now().UnixNano())
	n, err = c.Conn.Write(p)
	if n > 0 {
		c.bytesWritten.Add(int64(n))
	}
	return
}

func (c *LastUsageConn) Read(p []byte) (n int, err error) {
	c.lastUsageUnixNano.Store(time.Now().UnixNano())
	n, err = c.Conn.Read(p)
	if n > 0 {
		c.bytesRead.Add(int64(n))
	}
	return
}

func (c *LastUsageConn) LastUsage() time.Time {
	ns := c.lastUsageUnixNano.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

func (c *LastUsageConn) BytesRead() int64 {
	return c.bytesRead.Load()
}

func (c *LastUsageConn) BytesWritten() int64 {
	return c.bytesWritten.Load()
}
