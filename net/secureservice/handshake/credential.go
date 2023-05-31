package handshake

import (
	"context"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/libp2p/go-libp2p/core/sec"
)

func OutgoingHandshake(ctx context.Context, sc sec.SecureConn, cc CredentialChecker) (identity []byte, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	go func() {
		defer close(done)
		identity, err = outgoingHandshake(h, sc, cc)
	}()
	select {
	case <-done:
		return
	case <-ctx.Done():
		_ = sc.Close()
		return nil, ctx.Err()
	}
}

func outgoingHandshake(h *handshake, sc sec.SecureConn, cc CredentialChecker) (identity []byte, err error) {
	defer h.release()
	h.conn = sc
	localCred := cc.MakeCredentials(sc)
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

	if identity, err = cc.CheckCredential(sc, msg.cred); err != nil {
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

func IncomingHandshake(ctx context.Context, sc sec.SecureConn, cc CredentialChecker) (identity []byte, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h := newHandshake()
	done := make(chan struct{})
	go func() {
		defer close(done)
		identity, err = incomingHandshake(h, sc, cc)
	}()
	select {
	case <-done:
		return
	case <-ctx.Done():
		_ = sc.Close()
		return nil, ctx.Err()
	}
}

func incomingHandshake(h *handshake, sc sec.SecureConn, cc CredentialChecker) (identity []byte, err error) {
	defer h.release()
	h.conn = sc

	msg, err := h.readMsg(msgTypeCred)
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	if identity, err = cc.CheckCredential(sc, msg.cred); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}

	if err = h.writeCredentials(cc.MakeCredentials(sc)); err != nil {
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
