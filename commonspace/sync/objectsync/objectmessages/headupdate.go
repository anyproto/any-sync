package objectmessages

import (
	"fmt"
	"sync"

	"github.com/anyproto/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

var messagePool = &sync.Pool{
	New: func() interface{} {
		return &spacesyncproto.ObjectSyncMessage{}
	},
}

func NewMessage() *spacesyncproto.ObjectSyncMessage {
	return messagePool.Get().(*spacesyncproto.ObjectSyncMessage)
}

func FreeHeadUpdate(update *HeadUpdate) {
	if update.msg == nil {
		return
	}
	objMsg := update.msg
	update.Bytes = nil
	update.msg = nil
	messagePool.Put(objMsg)
}

type BroadcastOptions struct {
	EmptyPeers []string
}

type InnerHeadUpdate interface {
	Marshall(data ObjectMeta) ([]byte, error)
	Prepare() error
	Heads() []string
	MsgSize() uint64
	ObjectType() spacesyncproto.ObjectType
}

type ObjectMeta struct {
	PeerId   string
	ObjectId string
	SpaceId  string
}

type HeadUpdate struct {
	Meta       ObjectMeta
	Bytes      []byte
	Update     InnerHeadUpdate
	objectType spacesyncproto.ObjectType
	msg        *spacesyncproto.ObjectSyncMessage
}

func (h *HeadUpdate) MsgSize() uint64 {
	var byteSize uint64
	if h.Update != nil {
		byteSize += h.Update.MsgSize()
	} else {
		byteSize += uint64(len(h.Bytes))
	}
	return byteSize + uint64(len(h.Meta.PeerId)) + uint64(len(h.Meta.ObjectId)) + uint64(len(h.Meta.SpaceId))
}

func (h *HeadUpdate) Size() int {
	return int(h.MsgSize())
}

func (h *HeadUpdate) SetPeerId(peerId string) {
	h.Meta.PeerId = peerId
}

func (h *HeadUpdate) SetProtoMessage(message proto.Message) error {
	var (
		msg *spacesyncproto.ObjectSyncMessage
		ok  bool
	)
	if msg, ok = message.(*spacesyncproto.ObjectSyncMessage); !ok {
		return fmt.Errorf("unexpected message type: %T", message)
	}
	h.msg = msg
	h.Bytes = msg.GetPayload()
	h.Meta.SpaceId = msg.SpaceId
	h.Meta.ObjectId = msg.ObjectId
	h.objectType = msg.GetObjectType()
	return nil
}

func (h *HeadUpdate) ProtoMessage() (proto.Message, error) {
	if h.Update != nil {
		payload, err := h.Update.Marshall(h.Meta)
		if err != nil {
			return nil, err
		}
		return &spacesyncproto.ObjectSyncMessage{
			SpaceId:    h.Meta.SpaceId,
			Payload:    payload,
			ObjectId:   h.Meta.ObjectId,
			ObjectType: h.objectType,
		}, nil
	}
	return NewMessage(), nil
}

func (h *HeadUpdate) ObjectType() spacesyncproto.ObjectType {
	return h.objectType
}

func (h *HeadUpdate) SpaceId() string {
	return h.Meta.SpaceId
}

func (h *HeadUpdate) PeerId() string {
	return h.Meta.PeerId
}

func (h *HeadUpdate) ObjectId() string {
	return h.Meta.ObjectId
}

func (h *HeadUpdate) Copy() drpc.Message {
	return &HeadUpdate{
		Meta:       h.Meta,
		Bytes:      h.Bytes,
		Update:     h.Update,
		msg:        h.msg,
		objectType: h.objectType,
	}
}
