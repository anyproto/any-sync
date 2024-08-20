package response

import (
	"fmt"

	"github.com/anyproto/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type Response struct {
	SpaceId      string
	ObjectId     string
	Heads        []string
	SnapshotPath []string
	Changes      []*treechangeproto.RawTreeChangeWithId
	Root         *treechangeproto.RawTreeChangeWithId
}

func (r *Response) MsgSize() uint64 {
	size := uint64(len(r.SpaceId)+len(r.ObjectId)) * 59
	size += uint64(len(r.SnapshotPath)) * 59
	for _, change := range r.Changes {
		size += uint64(len(change.Id))
		size += uint64(len(change.RawChange))
	}
	return size + uint64(len(r.Heads))*59
}

func (r *Response) ProtoMessage() (proto.Message, error) {
	if r.ObjectId == "" {
		return &spacesyncproto.ObjectSyncMessage{}, nil
	}
	resp := &treechangeproto.TreeFullSyncResponse{
		Heads:        r.Heads,
		SnapshotPath: r.SnapshotPath,
		Changes:      r.Changes,
	}
	wrapped := treechangeproto.WrapFullResponse(resp, r.Root)
	return spacesyncproto.MarshallSyncMessage(wrapped, r.SpaceId, r.ObjectId)
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
	r.Root = treeMsg.RootChange
	headMsg := treeMsg.GetContent().GetFullSyncResponse()
	if headMsg == nil {
		return fmt.Errorf("unexpected message type: %T", treeMsg.GetContent())
	}
	r.Heads = headMsg.Heads
	r.Changes = headMsg.Changes
	r.SnapshotPath = headMsg.SnapshotPath
	r.SpaceId = msg.SpaceId
	r.ObjectId = msg.ObjectId
	return nil
}
