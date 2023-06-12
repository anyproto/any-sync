package connutil

import (
	"errors"
	"net"
	"os"
	"time"

	"github.com/anyproto/any-sync/app/logger"
	"go.uber.org/zap"
)

var log = logger.NewNamed("common.net.connutil")

type TimeoutConn struct {
	net.Conn
	timeout time.Duration
}

func NewTimeout(conn net.Conn, timeout time.Duration) *TimeoutConn {
	return &TimeoutConn{conn, timeout}
}

func (c *TimeoutConn) Write(p []byte) (n int, retErr error) {
	log.Debug("start write", zap.Int("n", len(p)))
	defer func() {
		if retErr != nil {
			log.Debug("conn write error", zap.Int("n", n), zap.Error(retErr))
		}
	}()
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
		log.Debug("conn write", zap.Int("n", n))
		return n, err
	}
}
