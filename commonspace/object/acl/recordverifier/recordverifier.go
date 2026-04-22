package recordverifier

import (
	"fmt"
	"slices"

	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

type AcceptorVerifier interface {
	VerifyAcceptor(rec *consensusproto.RawRecord) (err error)
	ShouldValidate() bool
}

type RecordVerifier = AcceptorVerifier

// ConsensusPeersSource is the minimal view of nodeconf needed by the verifier.
// nodeconf.NodeConf satisfies this interface; tests can supply a trivial fake.
type ConsensusPeersSource interface {
	ConsensusPeers() []string
}

// New returns a RecordVerifier that accepts only records whose AcceptorIdentity
// resolves to a peerId listed by src.ConsensusPeers(). Records signed by any
// other key — even with a well-formed Ed25519 signature — are rejected. The
// consensus node is assumed to use the same Ed25519 key for its peer identity
// and record acceptance (see any-sync-consensusnode/account/service.go).
func New(src ConsensusPeersSource) RecordVerifier {
	return &recordVerifier{
		store: crypto.NewKeyStorage(),
		src:   src,
	}
}

type recordVerifier struct {
	store crypto.KeyStorage
	src   ConsensusPeersSource
}

func (r *recordVerifier) VerifyAcceptor(rec *consensusproto.RawRecord) (err error) {
	identity, err := r.store.PubKeyFromProto(rec.AcceptorIdentity)
	if err != nil {
		identity, err = crypto.UnmarshalEd25519PublicKey(rec.AcceptorIdentity)
		if err != nil {
			return fmt.Errorf("failed to get acceptor identity: %w", err)
		}
	}
	peerId := identity.PeerId()
	if !slices.Contains(r.src.ConsensusPeers(), peerId) {
		return fmt.Errorf("acceptor %s is not a consensus node", peerId)
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
