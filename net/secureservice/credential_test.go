package secureservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/testutil/accounttest"
)

func TestPeerSignVerifier_CheckCredential(t *testing.T) {
	a1 := newTestAccData(t)
	a2 := newTestAccData(t)
	identity1, _ := a1.SignKey.GetPublic().Marshall()
	identity2, _ := a2.SignKey.GetPublic().Marshall()

	cc1 := newPeerSignVerifier(1, []uint32{1}, "test:v1", a1)
	cc2 := newPeerSignVerifier(1, []uint32{1}, "test:v1", a2)

	c1 := a2.PeerId
	c2 := a1.PeerId

	cr1 := cc1.MakeCredentials(c1)
	cr2 := cc2.MakeCredentials(c2)
	res, err := cc1.CheckCredential(c1, cr2)
	assert.NoError(t, err)
	assert.Equal(t, identity2, res.Identity)

	res2, err := cc2.CheckCredential(c2, cr1)
	assert.NoError(t, err)
	assert.Equal(t, identity1, res2.Identity)

	_, err = cc1.CheckCredential(c1, cr1)
	assert.EqualError(t, err, handshake.ErrInvalidCredentials.Error())
}

func TestIncompatibleVersion(t *testing.T) {
	a1 := newTestAccData(t)
	a2 := newTestAccData(t)
	_, _ = a1.SignKey.GetPublic().Marshall()

	cc1 := newPeerSignVerifier(0, []uint32{0}, "test:v1", a1)
	cc2 := newPeerSignVerifier(1, []uint32{1}, "test:v1", a2)

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

func TestIncompatibleVersion_Issue4423(t *testing.T) {
	a1 := newTestAccData(t)
	a2 := newTestAccData(t)
	identity2, _ := a2.SignKey.GetPublic().Marshall()

	cc1 := newPeerSignVerifier(1, []uint32{1}, "Linux:0.43.3/middle:v0.36.6/any-sync:v0.5.11", a1)
	cc2 := newPeerSignVerifier(1, []uint32{1}, "test:v1", a2)

	c1 := a2.PeerId
	c2 := a1.PeerId

	cr1 := cc1.MakeCredentials(c1)
	cr2 := cc2.MakeCredentials(c2)
	res, err := cc1.CheckCredential(c1, cr2)
	assert.NoError(t, err)
	assert.Equal(t, identity2, res.Identity)

	_, err = cc2.CheckCredential(c2, cr1)
	assert.ErrorIs(t, err, handshake.ErrIncompatibleVersion)
}

func TestIncompatibleVersion_BannedVersion(t *testing.T) {
	var test = func(bannedVersion string) {
		a1 := newTestAccData(t)
		a2 := newTestAccData(t)
		identity2, _ := a2.SignKey.GetPublic().Marshall()

		cc1 := newPeerSignVerifier(1, []uint32{1}, bannedVersion, a1)
		cc2 := newPeerSignVerifier(1, []uint32{1}, "test:v1", a2)

		c1 := a2.PeerId
		c2 := a1.PeerId

		cr1 := cc1.MakeCredentials(c1)
		cr2 := cc2.MakeCredentials(c2)
		res, err := cc1.CheckCredential(c1, cr2)
		assert.NoError(t, err)
		assert.Equal(t, identity2, res.Identity)

		_, err = cc2.CheckCredential(c2, cr1)
		assert.ErrorIs(t, err, handshake.ErrIncompatibleVersion, bannedVersion)
	}

	testBannedClients := []string{
		"Linux:0.43.3/middle:v0.36.6/any-sync:v0.5.11",
		"Mac:0.53.1/middle:v0.44.0-nightly.20251220.1/any-sync:v0.11.6",
	}
	for _, bannedVersion := range testBannedClients {
		test(bannedVersion)
	}

}

func newTestAccData(t *testing.T) *accountdata.AccountKeys {
	as := accounttest.AccountTestService{}
	require.NoError(t, as.Init(nil))
	return as.Account()
}
