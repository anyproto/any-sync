package handshake

import (
	"context"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"io"
)

func OutgoingHandshake(ctx context.Context, conn io.ReadWriteCloser, peerId string, cc CredentialChecker) (identity []byte, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	var (
		resIdentity []byte
		resErr      error
	)
	go func() {
		defer close(done)
		resIdentity, resErr = outgoingHandshake(h, conn, peerId, cc)
	}()
	select {
	case <-done:
		return resIdentity, resErr
	case <-ctx.Done():
		_ = conn.Close()
		return nil, ctx.Err()
	}
}

func outgoingHandshake(h *handshake, conn io.ReadWriteCloser, peerId string, cc CredentialChecker) (identity []byte, err error) {
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
			return nil, ErrPeerDeclinedCredentials
		}
		return nil, HandshakeError{e: msg.ack.Error}
	}

	if identity, err = cc.CheckCredential(peerId, msg.cred); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}

	if err = h.writeAck(handshakeproto.Error_Null); err != nil {
		h.tryWriteErrAndClose(err)
		return nil, err
	}

	msg, err = h.readMsg(msgTypeAck)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return nil, err
	}
	if msg.ack.Error == handshakeproto.Error_Null {
		return identity, nil
	} else {
		_ = h.conn.Close()
		return nil, HandshakeError{e: msg.ack.Error}
	}
}

func IncomingHandshake(ctx context.Context, conn io.ReadWriteCloser, peerId string, cc CredentialChecker) (identity []byte, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	var (
		resIdentity []byte
		resError    error
	)
	go func() {
		defer close(done)
		resIdentity, resError = incomingHandshake(h, conn, peerId, cc)
	}()
	select {
	case <-done:
		return resIdentity, resError
	case <-ctx.Done():
		_ = conn.Close()
		return nil, ctx.Err()
	}
}

func incomingHandshake(h *handshake, conn io.ReadWriteCloser, peerId string, cc CredentialChecker) (identity []byte, err error) {
	defer h.release()
	h.conn = conn

	msg, err := h.readMsg(msgTypeCred)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	if identity, err = cc.CheckCredential(peerId, msg.cred); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}

	if err = h.writeCredentials(cc.MakeCredentials(peerId)); err != nil {
		h.tryWriteErrAndClose(err)
		return nil, err
	}

	msg, err = h.readMsg(msgTypeAck)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return nil, err
	}
	if msg.ack.Error != handshakeproto.Error_Null {
		if msg.ack.Error == handshakeproto.Error_InvalidCredentials {
			return nil, ErrPeerDeclinedCredentials
		}
		return nil, HandshakeError{e: msg.ack.Error}
	}
	if err = h.writeAck(handshakeproto.Error_Null); err != nil {
		h.tryWriteErrAndClose(err)
		return nil, err
	}
	return
}
