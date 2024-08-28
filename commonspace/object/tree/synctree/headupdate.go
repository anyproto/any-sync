package synctree

import (
	"slices"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
)

type InnerHeadUpdate struct {
	opts          BroadcastOptions
	heads         []string
	changes       []*treechangeproto.RawTreeChangeWithId
	snapshotPath  []string
	root          *treechangeproto.RawTreeChangeWithId
	prepared      []byte
	emptyPrepared []byte
}

func (h *InnerHeadUpdate) MsgSize() (size uint64) {
	return uint64(len(h.prepared))
}

func (h *InnerHeadUpdate) Prepare() error {
	treeMsg := treechangeproto.WrapHeadUpdate(&treechangeproto.TreeHeadUpdate{
		Heads:        h.heads,
		SnapshotPath: h.snapshotPath,
		Changes:      h.changes,
	}, h.root)
	bytes, err := treeMsg.Marshal()
	if err != nil {
		return err
	}
	h.prepared = bytes
	emptyTreeMsg := treechangeproto.WrapHeadUpdate(&treechangeproto.TreeHeadUpdate{
		Heads:        h.heads,
		SnapshotPath: h.snapshotPath,
	}, h.root)
	bytes, err = emptyTreeMsg.Marshal()
	if err != nil {
		return err
	}
	h.changes = nil
	h.emptyPrepared = bytes
	return nil
}

func (h *InnerHeadUpdate) Marshall(data objectmessages.ObjectMeta) ([]byte, error) {
	if h.prepared != nil {
		if slices.Contains(h.opts.EmptyPeers, data.PeerId) {
			return h.emptyPrepared, nil
		} else {
			return h.prepared, nil
		}
	}
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

func (h *InnerHeadUpdate) Heads() []string {
	return h.heads
}
