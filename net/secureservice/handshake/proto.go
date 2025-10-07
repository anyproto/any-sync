package handshake

import (
	"context"
	"net"

	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
)

type ProtoChecker struct {
	AllowedProtoTypes  []handshakeproto.ProtoType
	SupportedEncodings []handshakeproto.Encoding
}

func OutgoingProtoHandshake(ctx context.Context, conn net.Conn, proto *handshakeproto.Proto) (*handshakeproto.Proto, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	var (
		err         error
		remoteProto *handshakeproto.Proto
	)
	go func() {
		defer close(done)
		remoteProto, err = outgoingProtoHandshake(h, conn, proto)
	}()
	select {
	case <-done:
		return remoteProto, err
	case <-ctx.Done():
		_ = conn.Close()
		return nil, ctx.Err()
	}
}

var noEncodings = []handshakeproto.Encoding{handshakeproto.Encoding_None}

func outgoingProtoHandshake(h *handshake, conn net.Conn, proto *handshakeproto.Proto) (remoteProto *handshakeproto.Proto, err error) {
	defer h.release()
	h.conn = conn
	localProto := proto
	if err = h.writeProto(localProto); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	msg, err := h.readMsg(msgTypeAck, msgTypeProto)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	// old clients with unsupported encodings will answer ack instead of proto
	if msg.ack != nil {
		if msg.ack.Error == handshakeproto.Error_IncompatibleProto {
			return nil, ErrRemoteIncompatibleProto
		}
		if msg.ack.Error == handshakeproto.Error_Null {
			return &handshakeproto.Proto{
				Proto:     proto.Proto,
				Encodings: noEncodings,
			}, nil
		} else {
			return nil, HandshakeError{e: msg.ack.Error}
		}
	} else if msg.proto != nil {
		return copyProto(msg.proto), nil
	} else {
		return nil, ErrUnexpectedPayload
	}
}

func IncomingProtoHandshake(ctx context.Context, conn net.Conn, pt ProtoChecker) (*handshakeproto.Proto, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	var (
		proto *handshakeproto.Proto
		err   error
	)
	go func() {
		defer close(done)
		proto, err = incomingProtoHandshake(h, conn, pt)
	}()
	select {
	case <-done:
		return proto, err
	case <-ctx.Done():
		_ = conn.Close()
		return nil, ctx.Err()
	}
}

func incomingProtoHandshake(h *handshake, conn net.Conn, pt ProtoChecker) (proto *handshakeproto.Proto, err error) {
	defer h.release()
	h.conn = conn

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

	// write ack for old clients without encodings support
	if len(msg.proto.Encodings) == 0 {
		if err = h.writeAck(handshakeproto.Error_Null); err != nil {
			h.tryWriteErrAndClose(err)
			return
		} else {
			return copyProto(msg.proto), nil
		}
	} else {
		enc := chooseEncoding(msg.proto.Encodings, pt.SupportedEncodings)
		if err = h.writeProto(&handshakeproto.Proto{
			Proto:     pt.AllowedProtoTypes[0],
			Encodings: enc,
		}); err != nil {
			h.tryWriteErrAndClose(err)
			return
		} else {
			return &handshakeproto.Proto{
				Proto:     msg.proto.Proto,
				Encodings: enc,
			}, nil
		}
	}
}

func chooseEncoding(remoteEncodings, localEncodings []handshakeproto.Encoding) (encodings []handshakeproto.Encoding) {
	for _, rEnc := range remoteEncodings {
		if slices.Contains(localEncodings, rEnc) {
			encodings = append(encodings, rEnc)
			return encodings
		}
	}
	return noEncodings
}

func copyProto(proto *handshakeproto.Proto) *handshakeproto.Proto {
	res := &handshakeproto.Proto{
		Proto:     proto.Proto,
		Encodings: make([]handshakeproto.Encoding, len(proto.Encodings)),
	}
	copy(res.Encodings, proto.Encodings)
	return res
}
