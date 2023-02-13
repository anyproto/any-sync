package timeoutconn

import (
	"errors"
	"github.com/anytypeio/any-sync/app/logger"
	"go.uber.org/zap"
	"net"
	"os"
	"time"
)

var log = logger.NewNamed("net.timeoutconn")

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
			if e := c.Conn.SetWriteDeadline(time.Now().Add(c.timeout)); e != nil {
				log.Warn("can't set write deadline", zap.String("remoteAddr", c.RemoteAddr().String()))
			}

		}
		nn, err := c.Conn.Write(p[n:])
		n += nn
		if n < len(p) && nn > 0 && errors.Is(err, os.ErrDeadlineExceeded) {
			// Keep extending the deadline so long as we're making progress.
			log.Debug("keep extending the deadline so long as we're making progress", zap.String("remoteAddr", c.RemoteAddr().String()))
			continue
		}
		if c.timeout != 0 {
			if e := c.Conn.SetWriteDeadline(time.Time{}); e != nil {
				log.Warn("can't set write deadline", zap.String("remoteAddr", c.RemoteAddr().String()))
			}
		}
		if err != nil {
			// if the connection is timed out and we should close it
			if e := c.Conn.Close(); e != nil {
				log.Warn("connection close error", zap.String("remoteAddr", c.RemoteAddr().String()))
			}
			log.Debug("connection timed out", zap.String("remoteAddr", c.RemoteAddr().String()))
		}
		return n, err
	}
}
