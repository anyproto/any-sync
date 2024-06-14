package syncacl

import (
	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

type Request struct {
	peerId   string
	objectId string
	spaceId  string
	head     string
	root     *consensusproto.RawRecordWithId
}

func NewRequest(peerId, objectId, spaceId, head string, root *consensusproto.RawRecordWithId) *Request {
	return &Request{
		peerId:   peerId,
		objectId: objectId,
		spaceId:  spaceId,
		head:     head,
		root:     root,
	}
}

func (r *Request) PeerId() string {
	return r.peerId
}

func (r *Request) ObjectId() string {
	return r.objectId
}

func (r *Request) Proto() (proto.Message, error) {
	req := &consensusproto.LogFullSyncRequest{
		Head: r.head,
	}
	fullSync := consensusproto.WrapFullRequest(req, r.root)
	return spacesyncproto.MarshallSyncMessage(fullSync, r.spaceId, r.objectId)
}
