package objectsync

import (
	"fmt"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/slice"
)

type RequestFactory interface {
	CreateHeadUpdate(t objecttree.ObjectTree, added []*treechangeproto.RawTreeChangeWithId) (msg *treechangeproto.TreeSyncMessage)
	CreateNewTreeRequest() (msg *treechangeproto.TreeSyncMessage)
	CreateFullSyncRequest(t objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (req *treechangeproto.TreeSyncMessage, err error)
	CreateFullSyncResponse(t objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (*treechangeproto.TreeSyncMessage, error)
}

func NewRequestFactory() RequestFactory {
	return &requestFactory{}
}

type requestFactory struct{}

func (r *requestFactory) CreateHeadUpdate(t objecttree.ObjectTree, added []*treechangeproto.RawTreeChangeWithId) (msg *treechangeproto.TreeSyncMessage) {
	return treechangeproto.WrapHeadUpdate(&treechangeproto.TreeHeadUpdate{
		Heads:        t.Heads(),
		Changes:      added,
		SnapshotPath: t.SnapshotPath(),
	}, t.Header())
}

func (r *requestFactory) CreateNewTreeRequest() (msg *treechangeproto.TreeSyncMessage) {
	return treechangeproto.WrapFullRequest(&treechangeproto.TreeFullSyncRequest{}, nil)
}

func (r *requestFactory) CreateFullSyncRequest(t objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (msg *treechangeproto.TreeSyncMessage, err error) {
	req := &treechangeproto.TreeFullSyncRequest{}
	if t == nil {
		return nil, fmt.Errorf("tree should not be empty")
	}

	req.Heads = t.Heads()
	req.SnapshotPath = t.SnapshotPath()

	var changesAfterSnapshot []*treechangeproto.RawTreeChangeWithId
	changesAfterSnapshot, err = t.ChangesAfterCommonSnapshot(theirSnapshotPath, theirHeads)
	if err != nil {
		return
	}

	req.Changes = changesAfterSnapshot
	msg = treechangeproto.WrapFullRequest(req, t.Header())
	return
}

func (r *requestFactory) CreateFullSyncResponse(t objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (msg *treechangeproto.TreeSyncMessage, err error) {
	resp := &treechangeproto.TreeFullSyncResponse{
		Heads:        t.Heads(),
		SnapshotPath: t.SnapshotPath(),
	}
	if slice.UnsortedEquals(theirHeads, t.Heads()) {
		msg = treechangeproto.WrapFullResponse(resp, t.Header())
		return
	}

	ourChanges, err := t.ChangesAfterCommonSnapshot(theirSnapshotPath, theirHeads)
	if err != nil {
		return
	}
	resp.Changes = ourChanges
	msg = treechangeproto.WrapFullResponse(resp, t.Header())
	return
}
