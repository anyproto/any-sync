package synctree

import (
	"slices"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
)

type InnerHeadUpdate struct {
	opts         BroadcastOptions
	heads        []string
	changes      []*treechangeproto.RawTreeChangeWithId
	snapshotPath []string
	root         *treechangeproto.RawTreeChangeWithId
}

func (h InnerHeadUpdate) Marshall(data objectmessages.ObjectMeta) ([]byte, error) {
	changes := h.changes
	if slices.Contains(h.opts.EmptyPeers, data.PeerId) {
		changes = nil
	}
	treeMsg := treechangeproto.WrapHeadUpdate(&treechangeproto.TreeHeadUpdate{
		Heads:        h.heads,
		SnapshotPath: h.snapshotPath,
		Changes:      changes,
	}, h.root)
	return treeMsg.Marshal()
}

type BroadcastOptions struct {
	EmptyPeers []string
}

func (h InnerHeadUpdate) Heads() []string {
	return h.heads
}
