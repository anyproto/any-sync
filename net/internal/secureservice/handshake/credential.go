package handshake

import (
	"context"
	"io"

	"github.com/anyproto/any-sync/net/internal/secureservice/handshake/handshakeproto"
	handshake2 "github.com/anyproto/any-sync/net/secureservice/handshake"
)

func OutgoingHandshake(ctx context.Context, conn io.ReadWriteCloser, peerId string, cc handshake2.CredentialChecker) (result handshake2.Result, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	var (
		res    handshake2.Result
		resErr error
	)
	go func() {
		defer close(done)
		res, resErr = outgoingHandshake(h, conn, peerId, cc)
	}()
	select {
	case <-done:
		return res, resErr
	case <-ctx.Done():
		_ = conn.Close()
		err = ctx.Err()
		return
	}
}

func outgoingHandshake(h *handshake, conn io.ReadWriteCloser, peerId string, cc handshake2.CredentialChecker) (result handshake2.Result, err error) {
	defer h.release()
	h.conn = conn
	localCred := cc.MakeCredentials(peerId)
	if err = h.writeCredentials(localCred); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	msg, err := h.readMsg(msgTypeAck, msgTypeCred)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	if msg.ack != nil {
		if msg.ack.Error == handshakeproto.Error_InvalidCredentials {
			err = handshake2.ErrPeerDeclinedCredentials
			return
		}
		err = handshake2.HandshakeError{E: msg.ack.Error}
		return
	}

	if result, err = cc.CheckCredential(peerId, msg.cred); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}

	if err = h.writeAck(handshakeproto.Error_Null); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}

	msg, err = h.readMsg(msgTypeAck)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	if msg.ack.Error == handshakeproto.Error_Null {
		return result, nil
	} else {
		_ = h.conn.Close()
		err = handshake2.HandshakeError{E: msg.ack.Error}
		return
	}
}

func IncomingHandshake(ctx context.Context, conn io.ReadWriteCloser, peerId string, cc handshake2.CredentialChecker) (result handshake2.Result, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	var (
		res      handshake2.Result
		resError error
	)
	go func() {
		defer close(done)
		res, resError = incomingHandshake(h, conn, peerId, cc)
	}()
	select {
	case <-done:
		return res, resError
	case <-ctx.Done():
		_ = conn.Close()
		err = ctx.Err()
		return
	}
}

func incomingHandshake(h *handshake, conn io.ReadWriteCloser, peerId string, cc handshake2.CredentialChecker) (result handshake2.Result, err error) {
	defer h.release()
	h.conn = conn

	msg, err := h.readMsg(msgTypeCred)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	if result, err = cc.CheckCredential(peerId, msg.cred); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}

	if err = h.writeCredentials(cc.MakeCredentials(peerId)); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}

	msg, err = h.readMsg(msgTypeAck)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	if msg.ack.Error != handshakeproto.Error_Null {
		if msg.ack.Error == handshakeproto.Error_InvalidCredentials {
			err = handshake2.ErrPeerDeclinedCredentials
			return
		}
		err = handshake2.HandshakeError{E: msg.ack.Error}
		return
	}
	if err = h.writeAck(handshakeproto.Error_Null); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	return
}
