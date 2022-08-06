package pool

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"go.uber.org/zap"
	"gopkg.in/mgo.v2/bson"
)

type Message struct {
	*syncproto.Message
	peer peer.Peer
}

func (m *Message) Peer() peer.Peer {
	return m.peer
}

func (m *Message) Reply(data []byte) (err error) {
	rep := &syncproto.Message{
		Header: &syncproto.Header{
			TraceId: m.GetHeader().TraceId,
			ReplyId: m.GetHeader().RequestId,
			Type:    syncproto.MessageType_MessageTypeSync,
		},
		Data: data,
	}
	return m.peer.Send(rep)
}

func (m *Message) Ack() (err error) {
	ack := &syncproto.System{
		Ack: &syncproto.SystemAck{},
	}
	data, err := ack.Marshal()
	if err != nil {
		return
	}
	rep := &syncproto.Message{
		Header: &syncproto.Header{
			TraceId:   m.GetHeader().TraceId,
			ReplyId:   m.GetHeader().RequestId,
			Type:      syncproto.MessageType_MessageTypeSystem,
			DebugInfo: "Ack",
		},
		Data: data,
	}
	err = m.peer.Send(rep)
	if err != nil {
		log.With(
			zap.String("peerId", m.peer.Id()),
			zap.String("header", rep.GetHeader().String())).
			Error("failed sending ack to peer", zap.Error(err))
	} else {
		log.With(
			zap.String("peerId", m.peer.Id()),
			zap.String("header", rep.GetHeader().String())).
			Debug("sent ack to peer")
	}
	return
}

func (m *Message) AckError(code syncproto.SystemErrorCode, description string) (err error) {
	ack := &syncproto.System{
		Ack: &syncproto.SystemAck{
			Error: &syncproto.SystemError{
				Code:        code,
				Description: description,
			},
		},
	}
	data, err := ack.Marshal()
	if err != nil {
		return
	}
	rep := &syncproto.Message{
		Header: &syncproto.Header{
			TraceId:   []byte(bson.NewObjectId()),
			ReplyId:   m.GetHeader().RequestId,
			Type:      syncproto.MessageType_MessageTypeSystem,
			DebugInfo: "AckError",
		},
		Data: data,
	}
	if err != nil {
		log.With(
			zap.String("peerId", m.peer.Id()),
			zap.String("header", rep.GetHeader().String())).
			Error("failed sending ackError to peer", zap.Error(err))
	} else {
		log.With(
			zap.String("peerId", m.peer.Id()),
			zap.String("header", rep.GetHeader().String())).
			Debug("sent ackError to peer")
	}
	return m.peer.Send(rep)
}
