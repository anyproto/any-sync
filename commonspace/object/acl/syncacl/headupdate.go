package syncacl

import (
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type InnerHeadUpdate struct {
	head    string
	records []*consensusproto.RawRecordWithId
	root    *consensusproto.RawRecordWithId
}

func (h InnerHeadUpdate) MsgSize() uint64 {
	size := uint64(len(h.head))
	for _, record := range h.records {
		size += uint64(len(record.Id))
		size += uint64(len(record.Payload))
	}
	return size + uint64(len(h.head)) + uint64(len(h.root.Id)) + uint64(len(h.root.Payload))
}

func (h InnerHeadUpdate) Marshall(data objectmessages.ObjectMeta) ([]byte, error) {
	treeMsg := consensusproto.WrapHeadUpdate(&consensusproto.LogHeadUpdate{
		Head:    h.head,
		Records: h.records,
	}, h.root)
	return treeMsg.Marshal()
}

func (h InnerHeadUpdate) Heads() []string {
	return []string{h.head}
}
