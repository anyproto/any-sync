package recordverifier

import (
	"fmt"

	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

type AcceptorVerifier interface {
	VerifyAcceptor(rec *consensusproto.RawRecord) (err error)
	ShouldValidate() bool
}

type RecordVerifier = AcceptorVerifier

// New returns a RecordVerifier that accepts only records whose AcceptorIdentity
// equals networkKey — the Ed25519 root key of the network (the pubkey encoded
// by nodeconf.Configuration().NetworkId; decode via crypto.DecodeNetworkId).
// Records signed by any other key — even with a well-formed Ed25519 signature —
// are rejected.
func New(networkKey crypto.PubKey) RecordVerifier {
	return &recordVerifier{
		store:      crypto.NewKeyStorage(),
		networkKey: networkKey,
	}
}

type recordVerifier struct {
	store      crypto.KeyStorage
	networkKey crypto.PubKey
}

func (r *recordVerifier) VerifyAcceptor(rec *consensusproto.RawRecord) (err error) {
	identity, err := r.store.PubKeyFromProto(rec.AcceptorIdentity)
	if err != nil {
		identity, err = crypto.UnmarshalEd25519PublicKey(rec.AcceptorIdentity)
		if err != nil {
			return fmt.Errorf("failed to get acceptor identity: %w", err)
		}
	}
	if r.networkKey == nil || !identity.Equals(r.networkKey) {
		return fmt.Errorf("acceptor %s is not the network key", identity.Network())
	}
	verified, err := identity.Verify(rec.Payload, rec.AcceptorSignature)
	if !verified || err != nil {
		return fmt.Errorf("failed to verify acceptor: %w", err)
	}
	return nil
}

func (r *recordVerifier) ShouldValidate() bool {
	return false
}
