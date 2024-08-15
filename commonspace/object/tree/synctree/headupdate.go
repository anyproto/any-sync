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

func (h InnerHeadUpdate) MsgSize() (size uint64) {
	for _, head := range h.heads {
		size += uint64(len(head))
	}
	for _, snapshotId := range h.snapshotPath {
		size += uint64(len(snapshotId))
	}
	for _, change := range h.changes {
		size += uint64(len(change.Id))
		size += uint64(len(change.RawChange))
	}
	return size
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
