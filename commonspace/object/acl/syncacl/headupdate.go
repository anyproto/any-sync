package syncacl

import (
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type InnerHeadUpdate struct {
	head     string
	records  []*consensusproto.RawRecordWithId
	root     *consensusproto.RawRecordWithId
	prepared []byte
}

func (h *InnerHeadUpdate) MsgSize() uint64 {
	if h.prepared != nil {
		return uint64(len(h.prepared))
	}
	size := uint64(len(h.head))
	for _, record := range h.records {
		size += uint64(len(record.Id))
		size += uint64(len(record.Payload))
	}
	return size + uint64(len(h.head)) + uint64(len(h.root.Id)) + uint64(len(h.root.Payload))
}

func (h *InnerHeadUpdate) Prepare() error {
	logMsg := consensusproto.WrapHeadUpdate(&consensusproto.LogHeadUpdate{
		Head:    h.head,
		Records: h.records,
	}, h.root)
	bytes, err := logMsg.Marshal()
	if err != nil {
		return err
	}
	h.records = nil
	h.prepared = bytes
	return nil
}

func (h *InnerHeadUpdate) Marshall(data objectmessages.ObjectMeta) ([]byte, error) {
	if h.prepared != nil {
		return h.prepared, nil
	}
	logMsg := consensusproto.WrapHeadUpdate(&consensusproto.LogHeadUpdate{
		Head:    h.head,
		Records: h.records,
	}, h.root)
	return logMsg.Marshal()
}

func (h *InnerHeadUpdate) Heads() []string {
	return []string{h.head}
}
