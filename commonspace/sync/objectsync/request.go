package objectsync

import (
	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type InnerRequest interface {
	Marshall() ([]byte, error)
}

type Request struct {
	peerId   string
	spaceId  string
	objectId string
	Inner    InnerRequest
	Bytes    []byte
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
