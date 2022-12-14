package conn

import (
	"errors"
	"net"
	"os"
	"time"
)

type Conn struct {
	net.Conn
	timeout time.Duration
}

func NewConn(conn net.Conn, timeout time.Duration) *Conn {
	return &Conn{conn, timeout}
}

func (c *Conn) Write(p []byte) (n int, err error) {
	for {
		if c.timeout != 0 {
			c.Conn.SetWriteDeadline(time.Now().Add(c.timeout))
		}
		nn, err := c.Conn.Write(p[n:])
		n += nn
		if n < len(p) && nn > 0 && errors.Is(err, os.ErrDeadlineExceeded) {
			// Keep extending the deadline so long as we're making progress.
			continue
		}
		if c.timeout != 0 {
			c.Conn.SetWriteDeadline(time.Time{})
		}
		if err != nil {
			c.Conn.Close()
		}
		return n, err
	}
}
