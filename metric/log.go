package metric

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/net/peer"
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

func PeerVersion(val string) zap.Field {
	return zap.String("peerVersion", val)
}

func Identity(val string) zap.Field {
	return zap.String("identity", val)
}

func App(app string) zap.Field {
	return zap.String("app", app)
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
	if m == nil {
		return
	}
	peerId, _ := peer.CtxPeerId(ctx)
	ak, _ := peer.CtxPubKey(ctx)
	var acc string
	if ak != nil {
		acc = ak.Account()
	}
	m.rpcLog.Info("", append(fields, m.appField, PeerId(peerId), Identity(acc), Method(rpc), PeerVersion(peer.CtxPeerClientVersion(ctx)))...)
}
