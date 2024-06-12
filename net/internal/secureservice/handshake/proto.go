package handshake

import (
	"context"
	"net"

	"github.com/anyproto/any-sync/net/internal/secureservice/handshake/handshakeproto"
	handshake2 "github.com/anyproto/any-sync/net/secureservice/handshake"

	"golang.org/x/exp/slices"
)

type ProtoChecker struct {
	AllowedProtoTypes []handshakeproto.ProtoType
}

func OutgoingProtoHandshake(ctx context.Context, conn net.Conn, pt handshakeproto.ProtoType) error {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		err = outgoingProtoHandshake(h, conn, pt)
	}()
	select {
	case <-done:
		return err
	case <-ctx.Done():
		_ = conn.Close()
		return ctx.Err()
	}
}

func outgoingProtoHandshake(h *handshake, conn net.Conn, pt handshakeproto.ProtoType) (err error) {
	defer h.release()
	h.conn = conn
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
		return handshake2.ErrRemoteIncompatibleProto
	}
	if msg.ack.Error == handshakeproto.Error_Null {
		return nil
	}
	return handshake2.HandshakeError{E: msg.ack.Error}
}

func IncomingProtoHandshake(ctx context.Context, conn net.Conn, pt ProtoChecker) (handshakeproto.ProtoType, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	var (
		protoType handshakeproto.ProtoType
		err       error
	)
	go func() {
		defer close(done)
		protoType, err = incomingProtoHandshake(h, conn, pt)
	}()
	select {
	case <-done:
		return protoType, err
	case <-ctx.Done():
		_ = conn.Close()
		return 0, ctx.Err()
	}
}

func incomingProtoHandshake(h *handshake, conn net.Conn, pt ProtoChecker) (protoType handshakeproto.ProtoType, err error) {
	defer h.release()
	h.conn = conn

	msg, err := h.readMsg(msgTypeProto)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	if !slices.Contains(pt.AllowedProtoTypes, msg.proto.Proto) {
		err = handshake2.ErrIncompatibleProto
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
