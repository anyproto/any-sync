package secureservice

import (
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/anytypeio/any-sync/net/secureservice/handshake"
	"github.com/anytypeio/any-sync/testutil/accounttest"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/sec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

func TestPeerSignVerifier_CheckCredential(t *testing.T) {
	a1 := newTestAccData(t)
	a2 := newTestAccData(t)

	cc1 := newPeerSignVerifier(a1)
	cc2 := newPeerSignVerifier(a2)

	c1 := newTestSC(a2.PeerId)
	c2 := newTestSC(a1.PeerId)

	cr1 := cc1.MakeCredentials(c1)
	cr2 := cc2.MakeCredentials(c2)
	id1, err := cc1.CheckCredential(c1, cr2)
	assert.NoError(t, err)
	assert.Equal(t, a2.Identity, id1)

	id2, err := cc2.CheckCredential(c2, cr1)
	assert.NoError(t, err)
	assert.Equal(t, a1.Identity, id2)

	_, err = cc1.CheckCredential(c1, cr1)
	assert.EqualError(t, err, handshake.ErrInvalidCredentials.Error())
}

func newTestAccData(t *testing.T) *accountdata.AccountKeys {
	as := accounttest.AccountTestService{}
	require.NoError(t, as.Init(nil))
	return as.Account()
}

func newTestSC(peerId string) sec.SecureConn {
	pid, _ := peer.Decode(peerId)
	return &testSc{
		ID: pid,
	}
}

type testSc struct {
	net.Conn
	peer.ID
}

func (t *testSc) LocalPeer() peer.ID {
	return ""
}

func (t *testSc) LocalPrivateKey() crypto.PrivKey {
	return nil
}

func (t *testSc) RemotePeer() peer.ID {
	return t.ID
}

func (t *testSc) RemotePublicKey() crypto.PubKey {
	return nil
}

func (t *testSc) ConnState() network.ConnectionState {
	return network.ConnectionState{}
}
