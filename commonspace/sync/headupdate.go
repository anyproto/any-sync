package sync

import (
	"fmt"
	"slices"

	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type HeadUpdate struct {
	peerId       string
	objectId     string
	spaceId      string
	heads        []string
	changes      []*treechangeproto.RawTreeChangeWithId
	snapshotPath []string
	root         *treechangeproto.RawTreeChangeWithId
	opts         BroadcastOptions
}

func (h *HeadUpdate) SetPeerId(peerId string) {
	h.peerId = peerId
}

func (h *HeadUpdate) SetProtoMessage(message proto.Message) error {
	var (
		msg *spacesyncproto.ObjectSyncMessage
		ok  bool
	)
	if msg, ok = message.(*spacesyncproto.ObjectSyncMessage); !ok {
		return fmt.Errorf("unexpected message type: %T", message)
	}
	treeMsg := &treechangeproto.TreeSyncMessage{}
	err := proto.Unmarshal(msg.Payload, treeMsg)
	if err != nil {
		return err
	}
	h.root = treeMsg.RootChange
	headMsg := treeMsg.GetContent().GetHeadUpdate()
	if headMsg == nil {
		return fmt.Errorf("unexpected message type: %T", treeMsg.GetContent())
	}
	h.heads = headMsg.Heads
	h.changes = headMsg.Changes
	h.snapshotPath = headMsg.SnapshotPath
	h.spaceId = msg.SpaceId
	h.objectId = msg.ObjectId
	return nil
}

func (h *HeadUpdate) ProtoMessage() (proto.Message, error) {
	if h.heads != nil {
		return h.SyncMessage()
	}
	return &spacesyncproto.ObjectSyncMessage{}, nil
}

func (h *HeadUpdate) PeerId() string {
	return h.peerId
}

func (h *HeadUpdate) ObjectId() string {
	return h.objectId
}

func (h *HeadUpdate) ShallowCopy() *HeadUpdate {
	return &HeadUpdate{
		peerId:       h.peerId,
		objectId:     h.objectId,
		heads:        h.heads,
		changes:      h.changes,
		snapshotPath: h.snapshotPath,
		root:         h.root,
	}
}

func (h *HeadUpdate) SyncMessage() (*spacesyncproto.ObjectSyncMessage, error) {
	changes := h.changes
	if slices.Contains(h.opts.EmptyPeers, h.peerId) {
		changes = nil
	}
	treeMsg := treechangeproto.WrapHeadUpdate(&treechangeproto.TreeHeadUpdate{
		Heads:        h.heads,
		SnapshotPath: h.snapshotPath,
		Changes:      changes,
	}, h.root)
	return spacesyncproto.MarshallSyncMessage(treeMsg, h.spaceId, h.objectId)
}

func (h *HeadUpdate) RemoveChanges() {
	h.changes = nil
}
