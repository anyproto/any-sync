package handshake

import (
	"context"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/libp2p/go-libp2p/core/sec"
	"golang.org/x/exp/slices"
)

type ProtoChecker struct {
	AllowedProtoTypes []handshakeproto.ProtoType
}

func OutgoingProtoHandshake(ctx context.Context, sc sec.SecureConn, pt handshakeproto.ProtoType) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	go func() {
		defer close(done)
		err = outgoingProtoHandshake(h, sc, pt)
	}()
	select {
	case <-done:
		return
	case <-ctx.Done():
		_ = sc.Close()
		return ctx.Err()
	}
}

func outgoingProtoHandshake(h *handshake, sc sec.SecureConn, pt handshakeproto.ProtoType) (err error) {
	defer h.release()
	h.conn = sc
	localProto := &handshakeproto.Proto{
		Proto: pt,
	}
	if err = h.writeProto(localProto); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	msg, err := h.readMsg(msgTypeAck)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	if msg.ack.Error == handshakeproto.Error_IncompatibleProto {
		return ErrRemoteIncompatibleProto
	}
	if msg.ack.Error == handshakeproto.Error_Null {
		return nil
	}
	return HandshakeError{e: msg.ack.Error}
}

func IncomingProtoHandshake(ctx context.Context, sc sec.SecureConn, pt ProtoChecker) (protoType handshakeproto.ProtoType, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	go func() {
		defer close(done)
		protoType, err = incomingProtoHandshake(h, sc, pt)
	}()
	select {
	case <-done:
		return
	case <-ctx.Done():
		_ = sc.Close()
		return 0, ctx.Err()
	}
}

func incomingProtoHandshake(h *handshake, sc sec.SecureConn, pt ProtoChecker) (protoType handshakeproto.ProtoType, err error) {
	defer h.release()
	h.conn = sc

	msg, err := h.readMsg(msgTypeProto)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	if !slices.Contains(pt.AllowedProtoTypes, msg.proto.Proto) {
		err = ErrIncompatibleProto
		h.tryWriteErrAndClose(err)
		return
	}

	if err = h.writeAck(handshakeproto.Error_Null); err != nil {
		h.tryWriteErrAndClose(err)
		return 0, err
	} else {
		return msg.proto.Proto, nil
	}
}
