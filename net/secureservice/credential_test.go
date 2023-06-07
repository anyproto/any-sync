package secureservice

import (
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPeerSignVerifier_CheckCredential(t *testing.T) {
	a1 := newTestAccData(t)
	a2 := newTestAccData(t)
	identity1, _ := a1.SignKey.GetPublic().Marshall()
	identity2, _ := a2.SignKey.GetPublic().Marshall()

	cc1 := newPeerSignVerifier(0, a1)
	cc2 := newPeerSignVerifier(0, a2)

	c1 := a2.PeerId
	c2 := a1.PeerId

	cr1 := cc1.MakeCredentials(c1)
	cr2 := cc2.MakeCredentials(c2)
	id1, err := cc1.CheckCredential(c1, cr2)
	assert.NoError(t, err)
	assert.Equal(t, identity2, id1)

	id2, err := cc2.CheckCredential(c2, cr1)
	assert.NoError(t, err)
	assert.Equal(t, identity1, id2)

	_, err = cc1.CheckCredential(c1, cr1)
	assert.EqualError(t, err, handshake.ErrInvalidCredentials.Error())
}

func TestIncompatibleVersion(t *testing.T) {
	a1 := newTestAccData(t)
	a2 := newTestAccData(t)
	_, _ = a1.SignKey.GetPublic().Marshall()

	cc1 := newPeerSignVerifier(0, a1)
	cc2 := newPeerSignVerifier(1, a2)

	c1 := a2.PeerId
	c2 := a1.PeerId

	cr1 := cc1.MakeCredentials(c1)
	cr2 := cc2.MakeCredentials(c2)
	_, err := cc1.CheckCredential(c1, cr2)
	assert.EqualError(t, err, handshake.ErrIncompatibleVersion.Error())

	_, err = cc2.CheckCredential(c2, cr1)
	assert.EqualError(t, err, handshake.ErrIncompatibleVersion.Error())

	_, err = cc1.CheckCredential(c1, cr1)
	assert.EqualError(t, err, handshake.ErrInvalidCredentials.Error())
}

func newTestAccData(t *testing.T) *accountdata.AccountKeys {
	as := accounttest.AccountTestService{}
	require.NoError(t, as.Init(nil))
	return as.Account()
}
