package handshake

import (
	"context"
	"encoding/binary"
	"errors"
	"github.com/anytypeio/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/libp2p/go-libp2p/core/sec"
	"golang.org/x/exp/slices"
	"io"
	"sync"
)

const headerSize = 5 // 1 byte for type + 4 byte for uint32 size

const (
	msgTypeCred = byte(1)
	msgTypeAck  = byte(2)
)

type HandshakeError struct {
	e handshakeproto.Error
}

func (he HandshakeError) Error() string {
	return he.e.String()
}

var (
	ErrUnexpectedPayload       = HandshakeError{handshakeproto.Error_UnexpectedPayload}
	ErrDeadlineExceeded        = HandshakeError{handshakeproto.Error_DeadlineExceeded}
	ErrInvalidCredentials      = HandshakeError{handshakeproto.Error_InvalidCredentials}
	ErrPeerDeclinedCredentials = errors.New("remote peer declined the credentials")
	ErrSkipVerifyNotAllowed    = HandshakeError{handshakeproto.Error_SkipVerifyNotAllowed}
	ErrUnexpected              = HandshakeError{handshakeproto.Error_Unexpected}

	ErrIncompatibleVersion = HandshakeError{handshakeproto.Error_IncompatibleVersion}

	ErrGotNotAHandshakeMessage = errors.New("go not a handshake message")
)

var handshakePool = &sync.Pool{New: func() any {
	return &handshake{
		remoteCred: &handshakeproto.Credentials{},
		remoteAck:  &handshakeproto.Ack{},
		localAck:   &handshakeproto.Ack{},
		buf:        make([]byte, 0, 1024),
	}
}}

type CredentialChecker interface {
	MakeCredentials(sc sec.SecureConn) *handshakeproto.Credentials
	CheckCredential(sc sec.SecureConn, cred *handshakeproto.Credentials) (identity []byte, err error)
}

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
	msg, err := h.readMsg()
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

	msg, err = h.readMsg()
	if err != nil {
		h.tryWriteErrAndClose(err)
		return nil, err
	}
	if msg.ack == nil {
		err = ErrUnexpectedPayload
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

	msg, err := h.readMsg()
	if err != nil {
		h.tryWriteErrAndClose(err)
		return
	}
	if msg.ack != nil {
		return nil, ErrUnexpectedPayload
	}
	if identity, err = cc.CheckCredential(sc, msg.cred); err != nil {
		h.tryWriteErrAndClose(err)
		return
	}

	if err = h.writeCredentials(cc.MakeCredentials(sc)); err != nil {
		h.tryWriteErrAndClose(err)
		return nil, err
	}

	msg, err = h.readMsg()
	if err != nil {
		h.tryWriteErrAndClose(err)
		return nil, err
	}
	if msg.ack == nil {
		err = ErrUnexpectedPayload
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

func newHandshake() *handshake {
	return handshakePool.Get().(*handshake)
}

type handshake struct {
	conn       sec.SecureConn
	remoteCred *handshakeproto.Credentials
	remoteAck  *handshakeproto.Ack
	localAck   *handshakeproto.Ack
	buf        []byte
}

func (h *handshake) writeCredentials(cred *handshakeproto.Credentials) (err error) {
	h.buf = slices.Grow(h.buf, cred.Size()+headerSize)[:cred.Size()+headerSize]
	n, err := cred.MarshalToSizedBuffer(h.buf[headerSize:])
	if err != nil {
		return err
	}
	return h.writeData(msgTypeCred, n)
}

func (h *handshake) tryWriteErrAndClose(err error) {
	if err == ErrGotNotAHandshakeMessage {
		// if we got unexpected message - just close the connection
		_ = h.conn.Close()
		return
	}
	var ackErr handshakeproto.Error
	if he, ok := err.(HandshakeError); ok {
		ackErr = he.e
	} else {
		ackErr = handshakeproto.Error_Unexpected
	}
	_ = h.writeAck(ackErr)
	_ = h.conn.Close()
}

func (h *handshake) writeAck(ackErr handshakeproto.Error) (err error) {
	h.localAck.Error = ackErr
	h.buf = slices.Grow(h.buf, h.localAck.Size()+headerSize)[:h.localAck.Size()+headerSize]
	n, err := h.localAck.MarshalTo(h.buf[headerSize:])
	if err != nil {
		return err
	}
	return h.writeData(msgTypeAck, n)
}

func (h *handshake) writeData(tp byte, size int) (err error) {
	h.buf[0] = tp
	binary.LittleEndian.PutUint32(h.buf[1:headerSize], uint32(size))
	_, err = h.conn.Write(h.buf[:size+headerSize])
	return err
}

type message struct {
	cred *handshakeproto.Credentials
	ack  *handshakeproto.Ack
}

func (h *handshake) readMsg() (msg message, err error) {
	h.buf = slices.Grow(h.buf, headerSize)[:headerSize]
	if _, err = io.ReadFull(h.conn, h.buf[:headerSize]); err != nil {
		return
	}
	tp := h.buf[0]
	if tp != msgTypeCred && tp != msgTypeAck {
		err = ErrGotNotAHandshakeMessage
		return
	}
	size := binary.LittleEndian.Uint32(h.buf[1:headerSize])
	h.buf = slices.Grow(h.buf, int(size))[:size]
	if _, err = io.ReadFull(h.conn, h.buf[:size]); err != nil {
		return
	}
	switch tp {
	case msgTypeCred:
		if err = h.remoteCred.Unmarshal(h.buf[:size]); err != nil {
			return
		}
		msg.cred = h.remoteCred
	case msgTypeAck:
		if err = h.remoteAck.Unmarshal(h.buf[:size]); err != nil {
			return
		}
		msg.ack = h.remoteAck
	}
	return
}

func (h *handshake) release() {
	h.buf = h.buf[:0]
	h.conn = nil
	h.localAck.Error = 0
	h.remoteAck.Error = 0
	h.remoteCred.Type = 0
	h.remoteCred.Payload = h.remoteCred.Payload[:0]
	handshakePool.Put(h)
}
