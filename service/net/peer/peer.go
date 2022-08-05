package peer

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"time"
)

type Dir uint

const (
	// DirInbound indicates peer created connection
	DirInbound Dir = iota
	// DirOutbound indicates that our host created  connection
	DirOutbound
)

type Info struct {
	Id             string
	Dir            Dir
	LastActiveUnix int64
}

func (i Info) LastActive() time.Time {
	return time.Unix(i.LastActiveUnix, 0)
}

type Peer interface {
	Id() string
	Info() Info
	Recv() (*syncproto.Message, error)
	Send(msg *syncproto.Message) (err error)
	Context() context.Context
	Close() error
}
