package handshake

import (
	"context"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	crypto2 "github.com/anyproto/any-sync/util/crypto"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/sec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
)

var noVerifyChecker = &testCredChecker{
	makeCred: &handshakeproto.Credentials{Type: handshakeproto.CredentialsType_SkipVerify},
	checkCred: func(sc sec.SecureConn, cred *handshakeproto.Credentials) (identity []byte, err error) {
		return []byte("identity"), nil
	},
}

type handshakeRes struct {
	identity []byte
	err      error
}

func TestOutgoingHandshake(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := OutgoingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		msg, err := h.readMsg(msgTypeCred, msgTypeAck, msgTypeProto)
		require.NoError(t, err)
		_, err = noVerifyChecker.CheckCredential(c2, msg.cred)
		require.NoError(t, err)
		// send credential message
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// receive ack
		msg, err = h.readMsg(msgTypeAck)
		require.NoError(t, err)
		require.Equal(t, handshakeproto.Error_Null, msg.ack.Error)
		// send ack
		require.NoError(t, h.writeAck(handshakeproto.Error_Null))
		res := <-handshakeResCh
		assert.NotEmpty(t, res.identity)
		assert.NoError(t, res.err)
	})
	t.Run("write cred err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := OutgoingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		_ = c2.Close()
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("read cred err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := OutgoingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		_ = c2.Close()
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("ack err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := OutgoingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		require.NoError(t, h.writeAck(ErrInvalidCredentials.e))
		res := <-handshakeResCh
		require.EqualError(t, res.err, ErrPeerDeclinedCredentials.Error())
	})
	t.Run("cred err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := OutgoingHandshake(nil, c1, &testCredChecker{makeCred: noVerifyChecker.makeCred, checkErr: ErrInvalidCredentials})
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		msg, err := h.readMsg(msgTypeAck)
		require.NoError(t, err)
		assert.Equal(t, ErrInvalidCredentials.e, msg.ack.Error)
		res := <-handshakeResCh
		require.EqualError(t, res.err, ErrInvalidCredentials.Error())
	})
	t.Run("write ack err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := OutgoingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		// write credentials and close conn
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		_ = c2.Close()
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("read ack err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := OutgoingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// read ack and close conn
		_, err = h.readMsg(msgTypeAck)
		require.NoError(t, err)
		_ = c2.Close()
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("write cred instead ack", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := OutgoingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// read ack
		_, err = h.readMsg(msgTypeAck)
		require.NoError(t, err)
		// write cred instead ack
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		_, err = h.readMsg(msgTypeAck)
		require.Error(t, err)
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("final ack error", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := OutgoingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		msg, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		_, err = noVerifyChecker.CheckCredential(c2, msg.cred)
		require.NoError(t, err)
		// send credential message
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// receive ack
		msg, err = h.readMsg(msgTypeAck)
		require.NoError(t, err)
		require.Equal(t, handshakeproto.Error_Null, msg.ack.Error)
		// send ack
		require.NoError(t, h.writeAck(handshakeproto.Error_UnexpectedPayload))
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("context cancel", func(t *testing.T) {
		var ctx, ctxCancel = context.WithCancel(context.Background())

		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := OutgoingHandshake(ctx, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		ctxCancel()
		res := <-handshakeResCh
		assert.EqualError(t, res.err, context.Canceled.Error())
		_, err = c2.Read(make([]byte, 10))
		assert.Error(t, err)
		_, err = c2.Write(make([]byte, 10))
		assert.Error(t, err)
	})
}

func TestIncomingHandshake(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// wait credentials
		msg, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		require.Equal(t, handshakeproto.CredentialsType_SkipVerify, msg.cred.Type)
		// write ack
		require.NoError(t, h.writeAck(handshakeproto.Error_Null))
		// wait ack
		msg, err = h.readMsg(msgTypeAck)
		require.NoError(t, err)
		assert.Equal(t, handshakeproto.Error_Null, msg.ack.Error)
		res := <-handshakeResCh
		assert.NotEmpty(t, res.identity)
		require.NoError(t, res.err)
	})
	t.Run("write cred err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		_ = c2.Close()
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("read cred err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials and close conn
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		_ = c2.Close()
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("write ack instead cred", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write ack instead cred
		require.NoError(t, h.writeAck(handshakeproto.Error_Null))
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("invalid cred", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, &testCredChecker{makeCred: noVerifyChecker.makeCred, checkErr: ErrInvalidCredentials})
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// except ack with error
		msg, err := h.readMsg(msgTypeAck)
		require.NoError(t, err)
		require.Nil(t, msg.cred)
		require.Equal(t, handshakeproto.Error_InvalidCredentials, msg.ack.Error)

		res := <-handshakeResCh
		require.EqualError(t, res.err, ErrInvalidCredentials.Error())
	})
	t.Run("invalid cred version", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, &testCredChecker{makeCred: noVerifyChecker.makeCred, checkErr: ErrIncompatibleVersion})
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// except ack with error
		msg, err := h.readMsg(msgTypeAck)
		require.NoError(t, err)
		require.Nil(t, msg.cred)
		require.Equal(t, handshakeproto.Error_IncompatibleVersion, msg.ack.Error)

		res := <-handshakeResCh
		assert.Equal(t, res.err, ErrIncompatibleVersion)
	})
	t.Run("write cred instead ack", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// read cred
		_, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		// write cred instead ack
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// expect EOF
		_, err = h.readMsg(msgTypeAck)
		require.Error(t, err)
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("read ack err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// read cred and close conn
		_, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		_ = c2.Close()

		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("write ack with invalid", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// wait credentials
		msg, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		require.Equal(t, handshakeproto.CredentialsType_SkipVerify, msg.cred.Type)
		// write ack
		require.NoError(t, h.writeAck(handshakeproto.Error_InvalidCredentials))

		res := <-handshakeResCh
		assert.EqualError(t, res.err, ErrPeerDeclinedCredentials.Error())
	})
	t.Run("write ack with err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// wait credentials
		msg, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		require.Equal(t, handshakeproto.CredentialsType_SkipVerify, msg.cred.Type)
		// write ack
		require.NoError(t, h.writeAck(handshakeproto.Error_Unexpected))

		res := <-handshakeResCh
		assert.EqualError(t, res.err, ErrUnexpected.Error())
	})
	t.Run("final ack error", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(nil, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// wait credentials
		msg, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		require.Equal(t, handshakeproto.CredentialsType_SkipVerify, msg.cred.Type)
		// write ack and close conn
		require.NoError(t, h.writeAck(handshakeproto.Error_Null))
		_ = c2.Close()
		res := <-handshakeResCh
		require.Error(t, res.err)
	})
	t.Run("context cancel", func(t *testing.T) {
		var ctx, ctxCancel = context.WithCancel(context.Background())
		c1, c2 := newConnPair(t)
		var handshakeResCh = make(chan handshakeRes, 1)
		go func() {
			identity, err := IncomingHandshake(ctx, c1, noVerifyChecker)
			handshakeResCh <- handshakeRes{identity: identity, err: err}
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// wait credentials
		_, err := h.readMsg(msgTypeCred)
		require.NoError(t, err)
		ctxCancel()
		res := <-handshakeResCh
		require.EqualError(t, res.err, context.Canceled.Error())
		_, err = c2.Read(make([]byte, 10))
		assert.Error(t, err)
		_, err = c2.Write(make([]byte, 10))
		assert.Error(t, err)
	})
}

func TestNotAHandshakeMessage(t *testing.T) {
	c1, c2 := newConnPair(t)
	var handshakeResCh = make(chan handshakeRes, 1)
	go func() {
		identity, err := IncomingHandshake(nil, c1, noVerifyChecker)
		handshakeResCh <- handshakeRes{identity: identity, err: err}
	}()
	h := newHandshake()
	h.conn = c2
	_, err := c2.Write([]byte("some unexpected bytes"))
	require.Error(t, err)
	res := <-handshakeResCh
	assert.Error(t, res.err)
}

func TestEndToEnd(t *testing.T) {
	c1, c2 := newConnPair(t)
	var (
		inResCh  = make(chan handshakeRes, 1)
		outResCh = make(chan handshakeRes, 1)
	)
	st := time.Now()
	go func() {
		identity, err := OutgoingHandshake(nil, c1, noVerifyChecker)
		outResCh <- handshakeRes{identity: identity, err: err}
	}()
	go func() {
		identity, err := IncomingHandshake(nil, c2, noVerifyChecker)
		inResCh <- handshakeRes{identity: identity, err: err}
	}()

	outRes := <-outResCh
	assert.NoError(t, outRes.err)
	assert.NotEmpty(t, outRes.identity)

	inRes := <-inResCh
	assert.NoError(t, inRes.err)
	assert.NotEmpty(t, inRes.identity)
	t.Log("dur", time.Since(st))
}

func BenchmarkHandshake(b *testing.B) {
	c1, c2 := newConnPair(b)
	var (
		inRes  = make(chan struct{})
		outRes = make(chan struct{})
		done   = make(chan struct{})
	)
	defer close(done)
	go func() {
		for {
			_, _ = OutgoingHandshake(nil, c1, noVerifyChecker)
			select {
			case outRes <- struct{}{}:
			case <-done:
				return
			}
		}
	}()
	go func() {
		for {
			_, _ = IncomingHandshake(nil, c2, noVerifyChecker)
			select {
			case inRes <- struct{}{}:
			case <-done:
				return
			}
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		<-outRes
		<-inRes
	}
}

type testCredChecker struct {
	makeCred  *handshakeproto.Credentials
	checkCred func(sc sec.SecureConn, cred *handshakeproto.Credentials) (identity []byte, err error)
	checkErr  error
}

func (t *testCredChecker) MakeCredentials(sc sec.SecureConn) *handshakeproto.Credentials {
	return t.makeCred
}

func (t *testCredChecker) CheckCredential(sc sec.SecureConn, cred *handshakeproto.Credentials) (identity []byte, err error) {
	if t.checkErr != nil {
		return nil, t.checkErr
	}
	if t.checkCred != nil {
		return t.checkCred(sc, cred)
	}
	return nil, nil
}

func newConnPair(t require.TestingT) (sc1, sc2 *secConn) {
	c1, c2 := net.Pipe()
	sk1, _, err := crypto2.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	sk1b, err := sk1.Raw()
	signKey1, err := crypto.UnmarshalEd25519PrivateKey(sk1b)
	require.NoError(t, err)
	sk2, _, err := crypto2.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	sk2b, err := sk2.Raw()
	signKey2, err := crypto.UnmarshalEd25519PrivateKey(sk2b)
	require.NoError(t, err)
	peerId1, err := crypto2.IdFromSigningPubKey(sk1.GetPublic())
	require.NoError(t, err)
	peerId2, err := crypto2.IdFromSigningPubKey(sk2.GetPublic())
	require.NoError(t, err)
	sc1 = &secConn{
		Conn:       c1,
		localKey:   signKey1,
		remotePeer: peerId2,
	}
	sc2 = &secConn{
		Conn:       c2,
		localKey:   signKey2,
		remotePeer: peerId1,
	}
	return
}

type secConn struct {
	net.Conn
	localKey   crypto.PrivKey
	remotePeer peer.ID
}

func (s *secConn) LocalPeer() peer.ID {
	skB, _ := s.localKey.Raw()
	sk, _ := crypto2.NewSigningEd25519PubKeyFromBytes(skB)
	lp, _ := crypto2.IdFromSigningPubKey(sk)
	return lp
}

func (s *secConn) LocalPrivateKey() crypto.PrivKey {
	return s.localKey
}

func (s *secConn) RemotePeer() peer.ID {
	return s.remotePeer
}

func (s *secConn) RemotePublicKey() crypto.PubKey {
	return nil
}

func (s *secConn) ConnState() network.ConnectionState {
	return network.ConnectionState{}
}
