package syncacl

import (
	"github.com/anyproto/any-sync/commonspace/sync/objectsync"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type InnerHeadUpdate struct {
	head    string
	records []*consensusproto.RawRecordWithId
	root    *consensusproto.RawRecordWithId
}

func (h InnerHeadUpdate) Marshall(data objectsync.ObjectMeta) ([]byte, error) {
	treeMsg := consensusproto.WrapHeadUpdate(&consensusproto.LogHeadUpdate{
		Head:    h.head,
		Records: h.records,
	}, h.root)
	return treeMsg.Marshal()
}

func (h InnerHeadUpdate) Heads() []string {
	return []string{h.head}
}
