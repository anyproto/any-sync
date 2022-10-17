package syncservice

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/slice"
)

type RequestFactory interface {
	CreateHeadUpdate(t tree.ObjectTree, added []*treechangeproto.RawTreeChangeWithId) (msg *spacesyncproto.ObjectSyncMessage)
	CreateNewTreeRequest(id string) (msg *spacesyncproto.ObjectSyncMessage)
	CreateFullSyncRequest(t tree.ObjectTree, theirHeads, theirSnapshotPath []string, trackingId string) (req *spacesyncproto.ObjectSyncMessage, err error)
	CreateFullSyncResponse(t tree.ObjectTree, theirHeads, theirSnapshotPath []string, trackingId string) (*spacesyncproto.ObjectSyncMessage, error)
}

func newRequestFactory() RequestFactory {
	return &requestFactory{}
}

type requestFactory struct{}

func (r *requestFactory) CreateHeadUpdate(t tree.ObjectTree, added []*treechangeproto.RawTreeChangeWithId) (msg *spacesyncproto.ObjectSyncMessage) {
	return spacesyncproto.WrapHeadUpdate(&spacesyncproto.ObjectHeadUpdate{
		Heads:        t.Heads(),
		Changes:      added,
		SnapshotPath: t.SnapshotPath(),
	}, t.Header(), t.ID(), "")
}

func (r *requestFactory) CreateNewTreeRequest(id string) (msg *spacesyncproto.ObjectSyncMessage) {
	return spacesyncproto.WrapFullRequest(&spacesyncproto.ObjectFullSyncRequest{}, nil, id, "")
}

func (r *requestFactory) CreateFullSyncRequest(t tree.ObjectTree, theirHeads, theirSnapshotPath []string, trackingId string) (msg *spacesyncproto.ObjectSyncMessage, err error) {
	req := &spacesyncproto.ObjectFullSyncRequest{}
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
	msg = spacesyncproto.WrapFullRequest(req, t.Header(), t.ID(), trackingId)
	return
}

func (r *requestFactory) CreateFullSyncResponse(t tree.ObjectTree, theirHeads, theirSnapshotPath []string, trackingId string) (msg *spacesyncproto.ObjectSyncMessage, err error) {
	resp := &spacesyncproto.ObjectFullSyncResponse{
		Heads:        t.Heads(),
		SnapshotPath: t.SnapshotPath(),
	}
	if slice.UnsortedEquals(theirHeads, t.Heads()) {
		msg = spacesyncproto.WrapFullResponse(resp, t.Header(), t.ID(), trackingId)
		return
	}

	ourChanges, err := t.ChangesAfterCommonSnapshot(theirSnapshotPath, theirHeads)
	if err != nil {
		return
	}
	resp.Changes = ourChanges
	msg = spacesyncproto.WrapFullResponse(resp, t.Header(), t.ID(), trackingId)
	return
}
