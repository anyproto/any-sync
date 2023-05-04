package metric

import (
	"github.com/anytypeio/any-sync/net/peer"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"time"
)

func Method(val string) zap.Field {
	return zap.String("rpc", val)
}

func QueueDur(val time.Duration) zap.Field {
	return zap.Float64("queueDur", val.Seconds())
}

func TotalDur(val time.Duration) zap.Field {
	return zap.Float64("totalDur", val.Seconds())
}

func SpaceId(val string) zap.Field {
	return zap.String("spaceId", val)
}

func ObjectId(val string) zap.Field {
	return zap.String("objectId", val)
}

func PeerId(val string) zap.Field {
	return zap.String("peerId", val)
}

func Identity(val string) zap.Field {
	return zap.String("identity", val)
}

func Addr(val string) zap.Field {
	return zap.String("addr", val)
}

func FileId(fileId string) zap.Field {
	return zap.String("fileId", fileId)
}

func Cid(cid string) zap.Field {
	return zap.String("cid", cid)
}

func Size(size int) zap.Field {
	return zap.Int("size", size)
}

func (m *metric) RequestLog(ctx context.Context, rpc string, fields ...zap.Field) {
	peerId, _ := peer.CtxPeerId(ctx)
	ak, _ := peer.CtxPubKey(ctx)
	var acc string
	if ak != nil {
		acc = ak.Account()
	}
	m.rpcLog.Info("", append(fields, Addr(peer.CtxPeerAddr(ctx)), PeerId(peerId), Identity(acc), Method(rpc))...)
}
