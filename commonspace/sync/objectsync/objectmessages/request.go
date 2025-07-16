package objectmessages

import (
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"google.golang.org/protobuf/proto"
)

type InnerRequest interface {
	Marshall() ([]byte, error)
	MsgSize() uint64
}

type Request struct {
	peerId   string
	spaceId  string
	objectId string
	Inner    InnerRequest
	Bytes    []byte
}

func (r *Request) MsgSize() uint64 {
	var byteSize uint64
	if r.Inner != nil {
		byteSize += r.Inner.MsgSize()
	} else {
		byteSize += uint64(len(r.Bytes))
	}
	return byteSize + uint64(len(r.peerId)) + uint64(len(r.objectId)) + uint64(len(r.spaceId))
}

func NewByteRequest(peerId, spaceId, objectId string, message []byte) *Request {
	return &Request{
		peerId:   peerId,
		spaceId:  spaceId,
		objectId: objectId,
		Bytes:    message,
	}
}

func NewRequest(peerId, spaceId, objectId string, inner InnerRequest) *Request {
	return &Request{
		peerId:   peerId,
		spaceId:  spaceId,
		objectId: objectId,
		Inner:    inner,
	}
}

func (r *Request) PeerId() string {
	return r.peerId
}

func (r *Request) ObjectId() string {
	return r.objectId
}

func (r *Request) Proto() (proto.Message, error) {
	msg, err := r.Inner.Marshall()
	if err != nil {
		return nil, err
	}
	return &spacesyncproto.ObjectSyncMessage{
		SpaceId:  r.spaceId,
		Payload:  msg,
		ObjectId: r.objectId,
	}, nil
}
