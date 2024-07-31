package syncacl

import (
	"fmt"

	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type Response struct {
	spaceId  string
	objectId string
	head     string
	records  []*consensusproto.RawRecordWithId
	root     *consensusproto.RawRecordWithId
}

func (r *Response) MsgSize() uint64 {
	size := uint64(len(r.head))
	for _, record := range r.records {
		size += uint64(len(record.Id))
		size += uint64(len(record.Payload))
	}
	return size + uint64(len(r.spaceId)) + uint64(len(r.objectId))
}

func (r *Response) ProtoMessage() (proto.Message, error) {
	if r.objectId == "" {
		return &spacesyncproto.ObjectSyncMessage{}, nil
	}
	resp := &consensusproto.LogFullSyncResponse{
		Head:    r.head,
		Records: r.records,
	}
	wrapped := consensusproto.WrapFullResponse(resp, r.root)
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
	logMsg := &consensusproto.LogSyncMessage{}
	err := proto.Unmarshal(msg.Payload, logMsg)
	if err != nil {
		return err
	}
	r.root = &consensusproto.RawRecordWithId{
		Payload: logMsg.Payload,
		Id:      logMsg.Id,
	}
	respMsg := logMsg.GetContent().GetFullSyncResponse()
	if respMsg == nil {
		return fmt.Errorf("unexpected message type: %T", logMsg.GetContent())
	}
	r.head = respMsg.Head
	r.records = respMsg.Records
	r.spaceId = msg.SpaceId
	r.objectId = msg.ObjectId
	return nil
}
