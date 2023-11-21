package streampool

import "storj.io/drpc"

type SizeableMessage interface {
	Size() int
}

type streamPoolStat struct {
	TotalSize int64        `json:"total_size"`
	Streams   []streamStat `json:"streams"`
}

type streamStat struct {
	PeerId    string `json:"peer_id"`
	MsgCount  int    `json:"msg_count"`
	TotalSize int64  `json:"total_size"`
}

func newStreamStat(peerId string) streamStat {
	return streamStat{
		PeerId: peerId,
	}
}

func (st *streamStat) AddMessage(msg drpc.Message) {
	if sizeable, ok := msg.(SizeableMessage); ok {
		st.TotalSize += int64(sizeable.Size())
		st.MsgCount++
	}
}

func (st *streamStat) RemoveMessage(msg drpc.Message) {
	if sizeable, ok := msg.(SizeableMessage); ok {
		st.TotalSize -= int64(sizeable.Size())
		st.MsgCount--
	}
}
