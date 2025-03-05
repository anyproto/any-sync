package response

import (
	"fmt"
	"google.golang.org/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type Response struct {
	SpaceId  string
	ObjectId string
	Head     string
	Records  []*consensusproto.RawRecordWithId
	Root     *consensusproto.RawRecordWithId
}

func (r *Response) MsgSize() uint64 {
	size := uint64(len(r.Head))
	for _, record := range r.Records {
		size += uint64(len(record.Id))
		size += uint64(len(record.Payload))
	}
	return size + uint64(len(r.SpaceId)) + uint64(len(r.ObjectId))
}

func (r *Response) ProtoMessage() (proto.Message, error) {
	if r.ObjectId == "" {
		return &spacesyncproto.ObjectSyncMessage{}, nil
	}
	resp := &consensusproto.LogFullSyncResponse{
		Head:    r.Head,
		Records: r.Records,
	}
	wrapped := consensusproto.WrapFullResponse(resp, r.Root)
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
	logMsg := &consensusproto.LogSyncMessage{}
	err := logMsg.UnmarshalVT(msg.Payload)
	if err != nil {
		return err
	}
	r.Root = &consensusproto.RawRecordWithId{
		Payload: logMsg.Payload,
		Id:      logMsg.Id,
	}
	respMsg := logMsg.GetContent().GetFullSyncResponse()
	if respMsg == nil {
		return fmt.Errorf("unexpected message type: %T", logMsg.GetContent())
	}
	r.Head = respMsg.Head
	r.Records = respMsg.Records
	r.SpaceId = msg.SpaceId
	r.ObjectId = msg.ObjectId
	return nil
}
