package synctree

import (
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
)

type InnerRequest struct {
	heads        []string
	snapshotPath []string
	root         *treechangeproto.RawTreeChangeWithId
}

func (r *InnerRequest) MsgSize() uint64 {
	size := uint64(len(r.heads)+len(r.snapshotPath)) * 59
	if r.root != nil {
		size += uint64(len(r.root.Id) + len(r.root.RawChange))
	}
	return size
}

func NewRequest(peerId, spaceId, objectId string, heads []string, snapshotPath []string, root *treechangeproto.RawTreeChangeWithId) *objectmessages.Request {
	return objectmessages.NewRequest(peerId, spaceId, objectId, &InnerRequest{
		heads:        heads,
		snapshotPath: snapshotPath,
		root:         root,
	})
}

func (r *InnerRequest) Marshall() ([]byte, error) {
	msg := &treechangeproto.TreeFullSyncRequest{
		Heads:        r.heads,
		SnapshotPath: r.snapshotPath,
	}
	req := treechangeproto.WrapFullRequest(msg, r.root)
	return req.Marshal()
}
