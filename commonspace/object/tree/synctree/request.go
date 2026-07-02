package synctree

import (
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
)

type InnerRequest struct {
	heads        []string
	snapshotPath []string
	root         *treechangeproto.RawTreeChangeWithId
	probe        bool
}

func (r *InnerRequest) MsgSize() uint64 {
	size := uint64(len(r.heads)+len(r.snapshotPath)) * 59
	if r.root != nil {
		size += uint64(len(r.root.Id) + len(r.root.RawChange))
	}
	return size
}

func NewRequest(peerId, spaceId, objectId string, heads []string, snapshotPath []string, root *treechangeproto.RawTreeChangeWithId) *objectmessages.Request {
	copyHeads := make([]string, len(heads))
	copy(copyHeads, heads)
	return objectmessages.NewRequest(peerId, spaceId, objectId, &InnerRequest{
		heads:        copyHeads,
		snapshotPath: snapshotPath,
		root:         root,
	})
}

// NewProbeRequest creates a request for the tree's root and current heads
// only — the responder sends no change bodies. Old responders ignore the
// probe flag and stream the full tree, so the caller's response collector
// must handle both shapes.
func NewProbeRequest(peerId, spaceId, objectId string) *objectmessages.Request {
	return objectmessages.NewRequest(peerId, spaceId, objectId, &InnerRequest{
		probe: true,
	})
}

func (r *InnerRequest) Marshall() ([]byte, error) {
	msg := &treechangeproto.TreeFullSyncRequest{
		Heads:        r.heads,
		SnapshotPath: r.snapshotPath,
		Probe:        r.probe,
	}
	req := treechangeproto.WrapFullRequest(msg, r.root)
	return req.MarshalVT()
}
