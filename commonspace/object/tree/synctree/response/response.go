package response

import (
	"fmt"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/protobuf"
)

type Response struct {
	SpaceId      string
	ObjectId     string
	Heads        []string
	SnapshotPath []string
	Changes      []*treechangeproto.RawTreeChangeWithId
	Root         *treechangeproto.RawTreeChangeWithId
}

const cidLen = 59

func (r *Response) MsgSize() uint64 {
	size := uint64(len(r.SpaceId) + len(r.ObjectId))
	size += uint64(len(r.SnapshotPath)) * cidLen
	for _, change := range r.Changes {
		size += uint64(len(change.Id))
		size += uint64(len(change.RawChange))
	}
	return size + uint64(len(r.Heads))*cidLen
}

func (r *Response) ProtoMessage() (protobuf.Message, error) {
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

func (r *Response) SetProtoMessage(message protobuf.Message) error {
	var (
		msg *spacesyncproto.ObjectSyncMessage
		ok  bool
	)
	if msg, ok = message.(*spacesyncproto.ObjectSyncMessage); !ok {
		return fmt.Errorf("unexpected message type: %T", message)
	}
	treeMsg := &treechangeproto.TreeSyncMessage{}
	err := treeMsg.UnmarshalVT(msg.Payload)
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
