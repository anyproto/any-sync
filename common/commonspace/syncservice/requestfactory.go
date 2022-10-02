package syncservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
)

type RequestFactory interface {
	FullSyncRequest(t tree.ObjectTree, theirHeads, theirSnapshotPath []string, trackingId string) (req *spacesyncproto.ObjectSyncMessage, err error)
	FullSyncResponse(t tree.ObjectTree, theirHeads, theirSnapshotPath []string, trackingId string) (*spacesyncproto.ObjectSyncMessage, error)
}

func newRequestFactory() RequestFactory {
	return &requestFactory{}
}

type requestFactory struct{}

func (r *requestFactory) FullSyncRequest(t tree.ObjectTree, theirHeads, theirSnapshotPath []string, trackingId string) (msg *spacesyncproto.ObjectSyncMessage, err error) {
	req := &spacesyncproto.ObjectFullSyncRequest{}
	if t == nil {
		msg = spacesyncproto.WrapFullRequest(req, t.Header(), t.ID(), trackingId)
		return
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

func (r *requestFactory) FullSyncResponse(t tree.ObjectTree, theirHeads, theirSnapshotPath []string, trackingId string) (msg *spacesyncproto.ObjectSyncMessage, err error) {
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
