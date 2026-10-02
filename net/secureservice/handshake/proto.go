package handshake

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
)

type ProtoChecker struct {
	AllowedProtoTypes  []handshakeproto.ProtoType
	SupportedEncodings []handshakeproto.Encoding
}

// OutgoingProtoHandshake negotiates the sub-connection protocol.
//
// Contract: on an I/O error or ctx cancellation the conn is closed; on a
// protocol-level error (incompatible, declined or unexpected proto) it is
// left to the caller. On cancellation the function returns at once and the
// close happens in the background: a stream close can block on the transport
// (a yamux FIN waits up to the connection write timeout), and the caller is
// typically racing a deadline. I/O errors close it asynchronously as well, so
// the conn may still be open when the function returns; it is unusable
// afterwards either way.
func OutgoingProtoHandshake(ctx context.Context, conn net.Conn, proto *handshakeproto.Proto) (*handshakeproto.Proto, error) {
	return OutgoingProtoHandshakeWithCloser(ctx, conn, proto, nil)
}

// OutgoingProtoHandshakeWithCloser is OutgoingProtoHandshake with every close
// of conn going through closeConn, which must not block (e.g. it hands the
// conn to a bounded cleanup worker). With a closer the conn is handed to it
// exactly once on any error, protocol-level ones included, so the caller
// never closes it itself. A nil closeConn keeps OutgoingProtoHandshake's
// contract and closes in a new goroutine.
func OutgoingProtoHandshakeWithCloser(ctx context.Context, conn net.Conn, proto *handshakeproto.Proto, closeConn func(net.Conn)) (*handshakeproto.Proto, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	closeOnAnyErr := closeConn != nil
	if closeConn == nil {
		closeConn = closeAsync
	} else {
		var handedOff atomic.Bool
		hook := closeConn
		closeConn = func(c net.Conn) {
			if handedOff.CompareAndSwap(false, true) {
				hook(c)
			}
		}
	}
	h := newHandshake()
	done := make(chan struct{})
	var (
		err         error
		remoteProto *handshakeproto.Proto
		// claimed is taken by whichever side finishes first: the handshake
		// goroutine (its result is returned) or the cancelled caller (which
		// then closes the conn itself)
		claimed atomic.Bool
	)
	go func() {
		defer close(done)
		remoteProto, err = outgoingProtoHandshake(h, conn, proto, closeConn, &claimed)
		if err != nil && closeOnAnyErr {
			// a no-op if the handshake or an abandoning caller closed it
			closeConn(conn)
		}
		claimed.CompareAndSwap(false, true)
	}()
	select {
	case <-done:
		return remoteProto, err
	case <-ctx.Done():
		if !claimed.CompareAndSwap(false, true) {
			// the handshake finished first and is returning right now
			<-done
			return remoteProto, err
		}
		// The deadline unblocks a pending read at once where supported; the
		// close is what reliably ends the handshake everywhere (a yamux write
		// waiting for the send loop ignores deadlines, and some conns have
		// no deadlines at all). Neither blocks the caller.
		_ = conn.SetDeadline(time.Now())
		closeConn(conn)
		return nil, ctx.Err()
	}
}

func closeAsync(conn net.Conn) {
	go func() { _ = conn.Close() }()
}

var noEncodings = []handshakeproto.Encoding{handshakeproto.Encoding_None}

func outgoingProtoHandshake(h *handshake, conn net.Conn, proto *handshakeproto.Proto, closeConn func(net.Conn), abandoned *atomic.Bool) (remoteProto *handshakeproto.Proto, err error) {
	defer h.release()
	h.conn = conn
	h.closeConn = closeConn
	h.abandoned = abandoned
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
