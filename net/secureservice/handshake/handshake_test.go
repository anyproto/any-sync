package handshake

import (
	"github.com/anytypeio/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
	peer2 "github.com/anytypeio/any-sync/util/peer"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/sec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	_ "net/http/pprof"
	"testing"
	"time"
)

var noVerifyChecker = &testCredChecker{
	makeCred: &handshakeproto.Credentials{Type: handshakeproto.CredentialsType_SkipVerify},
	checkCred: func(sc sec.SecureConn, cred *handshakeproto.Credentials) (err error) {
		return
	},
}

func TestOutgoingHandshake(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- OutgoingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		msg, err := h.readMsg()
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		require.NoError(t, noVerifyChecker.CheckCredential(c2, msg.cred))
		// send credential message
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// receive ack
		msg, err = h.readMsg()
		require.NoError(t, err)
		require.Equal(t, handshakeproto.Error_Null, msg.ack.Error)
		// send ack
		require.NoError(t, h.writeAck(handshakeproto.Error_Null))
		resErr := <-hanshareResCh
		assert.NoError(t, resErr)
	})
	t.Run("write cred err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- OutgoingHandshake(c1, noVerifyChecker)
		}()
		_ = c2.Close()
		require.Error(t, <-hanshareResCh)
	})
	t.Run("read cred err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- OutgoingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg()
		require.NoError(t, err)
		_ = c2.Close()
		require.Error(t, <-hanshareResCh)
	})
	t.Run("ack err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- OutgoingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg()
		require.NoError(t, err)
		require.NoError(t, h.writeAck(ErrInvalidCredentials.e))
		require.EqualError(t, <-hanshareResCh, ErrPeerDeclinedCredentials.Error())
	})
	t.Run("cred err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- OutgoingHandshake(c1, &testCredChecker{makeCred: noVerifyChecker.makeCred, checkErr: ErrInvalidCredentials})
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg()
		require.NoError(t, err)
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		msg, err := h.readMsg()
		require.NoError(t, err)
		assert.Equal(t, ErrInvalidCredentials.e, msg.ack.Error)
		require.EqualError(t, <-hanshareResCh, ErrInvalidCredentials.Error())
	})
	t.Run("write ack err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- OutgoingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg()
		require.NoError(t, err)
		// write credentials and close conn
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		_ = c2.Close()
		require.Error(t, <-hanshareResCh)
	})
	t.Run("read ack err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- OutgoingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg()
		require.NoError(t, err)
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// read ack and close conn
		_, err = h.readMsg()
		require.NoError(t, err)
		_ = c2.Close()
		require.Error(t, <-hanshareResCh)
	})
	t.Run("write cred instead ack", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- OutgoingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		_, err := h.readMsg()
		require.NoError(t, err)
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// read ack
		_, err = h.readMsg()
		require.NoError(t, err)
		// write cred instead ack
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		msg, err := h.readMsg()
		require.NoError(t, err)
		assert.Equal(t, handshakeproto.Error_UnexpectedPayload, msg.ack.Error)
		require.Error(t, <-hanshareResCh)
	})
	t.Run("final ack error", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- OutgoingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// receive credential message
		msg, err := h.readMsg()
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		require.NoError(t, noVerifyChecker.CheckCredential(c2, msg.cred))
		// send credential message
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// receive ack
		msg, err = h.readMsg()
		require.NoError(t, err)
		require.Equal(t, handshakeproto.Error_Null, msg.ack.Error)
		// send ack
		require.NoError(t, h.writeAck(handshakeproto.Error_UnexpectedPayload))
		resErr := <-hanshareResCh
		assert.Error(t, resErr)
	})
}

func TestIncomingHandshake(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- IncomingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// wait credentials
		msg, err := h.readMsg()
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		require.Equal(t, handshakeproto.CredentialsType_SkipVerify, msg.cred.Type)
		// write ack
		require.NoError(t, h.writeAck(handshakeproto.Error_Null))
		// wait ack
		msg, err = h.readMsg()
		require.NoError(t, err)
		assert.Equal(t, handshakeproto.Error_Null, msg.ack.Error)
		require.NoError(t, <-hanshareResCh)
	})
	t.Run("write cred err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- IncomingHandshake(c1, noVerifyChecker)
		}()
		_ = c2.Close()
		require.Error(t, <-hanshareResCh)
	})
	t.Run("read cred err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- IncomingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials and close conn
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		_ = c2.Close()
		require.Error(t, <-hanshareResCh)
	})
	t.Run("write ack instead cred", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- IncomingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// write ack instead cred
		require.NoError(t, h.writeAck(handshakeproto.Error_Null))
		require.Error(t, <-hanshareResCh)
	})
	t.Run("invalid cred", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- IncomingHandshake(c1, &testCredChecker{makeCred: noVerifyChecker.makeCred, checkErr: ErrInvalidCredentials})
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// except ack with error
		msg, err := h.readMsg()
		require.NoError(t, err)
		require.Nil(t, msg.cred)
		require.Equal(t, handshakeproto.Error_InvalidCredentials, msg.ack.Error)

		require.EqualError(t, <-hanshareResCh, ErrInvalidCredentials.Error())
	})
	t.Run("write cred instead ack", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- IncomingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// read cred
		_, err := h.readMsg()
		require.NoError(t, err)
		// write cred instead ack
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// expect ack with error
		msg, err := h.readMsg()
		require.Equal(t, handshakeproto.Error_UnexpectedPayload, msg.ack.Error)
		require.Error(t, <-hanshareResCh)
	})
	t.Run("read ack err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- IncomingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// read cred and close conn
		_, err := h.readMsg()
		require.NoError(t, err)
		_ = c2.Close()

		require.Error(t, <-hanshareResCh)
	})
	t.Run("write ack with invalid", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- IncomingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// wait credentials
		msg, err := h.readMsg()
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		require.Equal(t, handshakeproto.CredentialsType_SkipVerify, msg.cred.Type)
		// write ack
		require.NoError(t, h.writeAck(handshakeproto.Error_InvalidCredentials))

		assert.EqualError(t, <-hanshareResCh, ErrPeerDeclinedCredentials.Error())
	})
	t.Run("write ack with err", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- IncomingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// wait credentials
		msg, err := h.readMsg()
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		require.Equal(t, handshakeproto.CredentialsType_SkipVerify, msg.cred.Type)
		// write ack
		require.NoError(t, h.writeAck(handshakeproto.Error_Unexpected))

		assert.EqualError(t, <-hanshareResCh, ErrUnexpected.Error())
	})
	t.Run("final ack error", func(t *testing.T) {
		c1, c2 := newConnPair(t)
		var hanshareResCh = make(chan error, 1)
		go func() {
			hanshareResCh <- IncomingHandshake(c1, noVerifyChecker)
		}()
		h := newHandshake()
		h.conn = c2
		// write credentials
		require.NoError(t, h.writeCredentials(noVerifyChecker.MakeCredentials(c2)))
		// wait credentials
		msg, err := h.readMsg()
		require.NoError(t, err)
		require.Nil(t, msg.ack)
		require.Equal(t, handshakeproto.CredentialsType_SkipVerify, msg.cred.Type)
		// write ack and close conn
		require.NoError(t, h.writeAck(handshakeproto.Error_Null))
		_ = c2.Close()
		require.Error(t, <-hanshareResCh)
	})
}

func TestNotAHandshakeMessage(t *testing.T) {
	c1, c2 := newConnPair(t)
	var hanshareResCh = make(chan error, 1)
	go func() {
		hanshareResCh <- IncomingHandshake(c1, noVerifyChecker)
	}()
	h := newHandshake()
	h.conn = c2
	_, err := c2.Write([]byte("some unexpected bytes"))
	require.Error(t, err)
	assert.EqualError(t, <-hanshareResCh, ErrGotNotAHandshakeMessage.Error())
}

func TestEndToEnd(t *testing.T) {
	c1, c2 := newConnPair(t)
	var (
		inRes  = make(chan error, 1)
		outRes = make(chan error, 1)
	)
	st := time.Now()
	go func() {
		outRes <- OutgoingHandshake(c1, noVerifyChecker)
	}()
	go func() {
		inRes <- IncomingHandshake(c2, noVerifyChecker)
	}()
	assert.NoError(t, <-outRes)
	assert.NoError(t, <-inRes)
	t.Log("dur", time.Since(st))
}

func BenchmarkHandshake(b *testing.B) {
	c1, c2 := newConnPair(b)
	var (
		inRes  = make(chan error)
		outRes = make(chan error)
		done   = make(chan struct{})
	)
	defer close(done)
	go func() {
		for {
			select {
			case outRes <- OutgoingHandshake(c1, noVerifyChecker):
			case <-done:
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case inRes <- IncomingHandshake(c2, noVerifyChecker):
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
	checkCred func(sc sec.SecureConn, cred *handshakeproto.Credentials) (err error)
	checkErr  error
}

func (t *testCredChecker) MakeCredentials(sc sec.SecureConn) *handshakeproto.Credentials {
	return t.makeCred
}

func (t *testCredChecker) CheckCredential(sc sec.SecureConn, cred *handshakeproto.Credentials) (err error) {
	if t.checkErr != nil {
		return t.checkErr
	}
	if t.checkCred != nil {
		return t.checkCred(sc, cred)
	}
	return nil
}

func newConnPair(t require.TestingT) (sc1, sc2 *secConn) {
	c1, c2 := net.Pipe()
	sk1, _, err := signingkey.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	sk1b, err := sk1.Raw()
	signKey1, err := crypto.UnmarshalEd25519PrivateKey(sk1b)
	require.NoError(t, err)
	sk2, _, err := signingkey.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	sk2b, err := sk2.Raw()
	signKey2, err := crypto.UnmarshalEd25519PrivateKey(sk2b)
	require.NoError(t, err)
	peerId1, err := peer2.IdFromSigningPubKey(sk1.GetPublic())
	require.NoError(t, err)
	peerId2, err := peer2.IdFromSigningPubKey(sk2.GetPublic())
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
	sk, _ := signingkey.NewSigningEd25519PubKeyFromBytes(skB)
	lp, _ := peer2.IdFromSigningPubKey(sk)
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
