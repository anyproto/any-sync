package recordverifier

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/nodeconf/testconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/util/crypto"
)

type fixture struct {
	*recordVerifier
	app            *app.App
	networkPrivKey crypto.PrivKey
}

func newFixture(t *testing.T) *fixture {
	accService := &accounttest.AccountTestService{}
	a := &app.App{}
	verifier := &recordVerifier{}
	a.Register(accService).
		Register(&testconf.StubConf{}).
		Register(verifier)
	require.NoError(t, a.Start(context.Background()))
	return &fixture{
		recordVerifier: verifier,
		app:            a,
		networkPrivKey: accService.Account().SignKey,
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
