package recordverifier

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/util/crypto"
)

type fakeConsensusPeers struct {
	peers []string
}

func (f fakeConsensusPeers) ConsensusPeers() []string { return f.peers }

type fixture struct {
	*recordVerifier
	networkPrivKey crypto.PrivKey
}

func newFixture(t *testing.T) *fixture {
	accService := &accounttest.AccountTestService{}
	require.NoError(t, accService.Init(nil))
	networkKey := accService.Account().SignKey
	src := fakeConsensusPeers{peers: []string{networkKey.GetPublic().PeerId()}}
	verifier := New(src).(*recordVerifier)
	return &fixture{
		recordVerifier: verifier,
		networkPrivKey: networkKey,
	}
}

func TestRecordVerifier_VerifyAcceptor(t *testing.T) {
	fx := newFixture(t)
	identity, err := fx.networkPrivKey.GetPublic().Marshall()
	require.NoError(t, err)
	testPayload := []byte("test payload")
	acceptorSignature, err := fx.networkPrivKey.Sign(testPayload)
	require.NoError(t, err)
	rawRecord := &consensusproto.RawRecord{
		AcceptorIdentity:  identity,
		Payload:           testPayload,
		AcceptorSignature: acceptorSignature,
	}
	err = fx.VerifyAcceptor(rawRecord)
	require.NoError(t, err)
}

func TestRecordVerifier_VerifyAcceptor_InvalidSignature(t *testing.T) {
	fx := newFixture(t)
	identity, err := fx.networkPrivKey.GetPublic().Marshall()
	require.NoError(t, err)
	testPayload := []byte("test payload")
	rawRecord := &consensusproto.RawRecord{
		AcceptorIdentity:  identity,
		Payload:           testPayload,
		AcceptorSignature: []byte("invalid signature"),
	}
	err = fx.VerifyAcceptor(rawRecord)
	require.Error(t, err)
}

func TestRecordVerifier_VerifyAcceptor_ModifiedPayload(t *testing.T) {
	fx := newFixture(t)
	identity, err := fx.networkPrivKey.GetPublic().Marshall()
	require.NoError(t, err)
	testPayload := []byte("test payload")
	acceptorSignature, err := fx.networkPrivKey.Sign(testPayload)
	require.NoError(t, err)
	rawRecord := &consensusproto.RawRecord{
		AcceptorIdentity:  identity,
		Payload:           []byte("modified payload"),
		AcceptorSignature: acceptorSignature,
	}
	err = fx.VerifyAcceptor(rawRecord)
	require.Error(t, err)
}

func TestRecordVerifier_VerifyAcceptor_InvalidIdentity(t *testing.T) {
	fx := newFixture(t)
	testPayload := []byte("test payload")
	acceptorSignature, err := fx.networkPrivKey.Sign(testPayload)
	require.NoError(t, err)
	rawRecord := &consensusproto.RawRecord{
		AcceptorIdentity:  []byte("invalid identity"),
		Payload:           testPayload,
		AcceptorSignature: acceptorSignature,
	}
	err = fx.VerifyAcceptor(rawRecord)
	require.Error(t, err)
}

func TestRecordVerifier_VerifyAcceptor_EmptySignature(t *testing.T) {
	fx := newFixture(t)
	identity, err := fx.networkPrivKey.GetPublic().Marshall()
	require.NoError(t, err)
	rawRecord := &consensusproto.RawRecord{
		AcceptorIdentity:  identity,
		Payload:           []byte("test payload"),
		AcceptorSignature: nil,
	}
	err = fx.VerifyAcceptor(rawRecord)
	require.Error(t, err)
}

// An acceptor signature may be cryptographically valid, but if the signer's
// peerId is not in ConsensusPeers() the record must be rejected. This is the
// Cure53 finding the verifier was hardened against.
func TestRecordVerifier_VerifyAcceptor_UnknownAcceptor(t *testing.T) {
	fx := newFixture(t)
	roguePriv, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	identity, err := roguePriv.GetPublic().Marshall()
	require.NoError(t, err)
	testPayload := []byte("test payload")
	acceptorSignature, err := roguePriv.Sign(testPayload)
	require.NoError(t, err)
	rawRecord := &consensusproto.RawRecord{
		AcceptorIdentity:  identity,
		Payload:           testPayload,
		AcceptorSignature: acceptorSignature,
	}
	err = fx.VerifyAcceptor(rawRecord)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a consensus node")
}
