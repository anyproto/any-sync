package streampool

import (
	"sync/atomic"

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
		st.msgCount.Add(1)
	}
}

func (st *streamStat) RemoveMessage(msg drpc.Message) {
	if sizeable, ok := msg.(SizeableMessage); ok {
		st.totalSize.Add(-int64(sizeable.Size()))
		st.msgCount.Add(-1)
	}
}
