package streampool

import (
	"sync/atomic"

	"go.uber.org/zap"
	"storj.io/drpc"
)

type SizeableMessage interface {
	Size() int
}

type streamPoolStat struct {
	TotalSize int64        `json:"total_size"`
	Streams   []streamStat `json:"streams,omitempty"`
}

type streamStat struct {
	PeerId    string `json:"peer_id"`
	MsgCount  int    `json:"msg_count"`
	TotalSize int64  `json:"total_size"`
	msgCount  atomic.Int32
	totalSize atomic.Int64
}

func newStreamStat(peerId string) streamStat {
	return streamStat{
		PeerId: peerId,
	}
}

func (st *streamStat) AddMessage(msg drpc.Message) {
	if sizeable, ok := msg.(SizeableMessage); ok {
		st.totalSize.Add(int64(sizeable.Size()))
	}
	st.msgCount.Add(1)
}

func (st *streamStat) RemoveMessage(msg drpc.Message) {
	if sizeable, ok := msg.(SizeableMessage); ok {
		size := sizeable.Size()
		// TODO: find the real problem :-)
		if st.totalSize.Load() >= int64(size) {
			st.totalSize.Add(-int64(size))
		} else {
			log.Error("streamStat.RemoveMessage: totalSize is less than message size", zap.Int("size", size), zap.Int64("totalSize", st.totalSize.Load()))
			st.totalSize.Store(0)
		}
	}
	st.msgCount.Add(-1)
}
