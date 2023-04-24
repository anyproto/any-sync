package metric

import (
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"time"
)

func Method(val string) zap.Field {
	return zap.String("rpc", val)
}

func QueueDur(val time.Duration) zap.Field {
	return zap.Int64("queueMs", val.Milliseconds())
}

func TotalDur(val time.Duration) zap.Field {
	return zap.Int64("totalMs", val.Milliseconds())
}

func SpaceId(val string) zap.Field {
	return zap.String("spaceId", val)
}

func ObjectId(val string) zap.Field {
	return zap.String("objectId", val)
}

func Identity(val string) zap.Field {
	return zap.String("identity", val)
}

func IP(val string) zap.Field {
	return zap.String("ip", val)
}

func (m *metric) RequestLog(ctx context.Context, fields ...zap.Field) {
	m.rpcLog.InfoCtx(ctx, "", fields...)
}
