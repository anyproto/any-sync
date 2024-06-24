package synctree

import (
	"fmt"

	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type Response struct {
	spaceId      string
	objectId     string
	heads        []string
	snapshotPath []string
	changes      []*treechangeproto.RawTreeChangeWithId
	root         *treechangeproto.RawTreeChangeWithId
}

func (r *Response) MsgSize() uint64 {
	size := uint64(len(r.spaceId)+len(r.objectId)) * 59
	size += uint64(len(r.snapshotPath)) * 59
	for _, change := range r.changes {
		size += uint64(len(change.Id))
		size += uint64(len(change.RawChange))
	}
	return size + uint64(len(r.heads))*59
}

func (r *Response) ProtoMessage() (proto.Message, error) {
	resp := &treechangeproto.TreeFullSyncResponse{
		Heads:        r.heads,
		SnapshotPath: r.snapshotPath,
		Changes:      r.changes,
	}
	wrapped := treechangeproto.WrapFullResponse(resp, r.root)
	return spacesyncproto.MarshallSyncMessage(wrapped, r.spaceId, r.objectId)
}

func (r *Response) SetProtoMessage(message proto.Message) error {
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
	r.root = treeMsg.RootChange
	headMsg := treeMsg.GetContent().GetFullSyncResponse()
	if headMsg == nil {
		return fmt.Errorf("unexpected message type: %T", treeMsg.GetContent())
	}
	r.heads = headMsg.Heads
	r.changes = headMsg.Changes
	r.snapshotPath = headMsg.SnapshotPath
	r.spaceId = msg.SpaceId
	r.objectId = msg.ObjectId
	return nil
}
