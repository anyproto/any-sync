package sync

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/metric"
)

type HandleMessage struct {
	Id                uint64
	ReceiveTime       time.Time
	StartHandlingTime time.Time
	Deadline          time.Time
	SenderId          string
	Message           *spacesyncproto.ObjectSyncMessage
	PeerCtx           context.Context
}

func (m HandleMessage) LogFields(fields ...zap.Field) []zap.Field {
	return append(fields,
		metric.SpaceId(m.Message.SpaceId),
		metric.ObjectId(m.Message.ObjectId),
		metric.QueueDur(m.StartHandlingTime.Sub(m.ReceiveTime)),
		metric.TotalDur(time.Since(m.ReceiveTime)),
	)
}
