package recordverifier

import (
	"fmt"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/crypto"
)

const CName = "common.acl.recordverifier"

type AcceptorVerifier interface {
	VerifyAcceptor(rec *consensusproto.RawRecord) (err error)
	ShouldValidate() bool
}

type RecordVerifier interface {
	app.Component
	AcceptorVerifier
}

func New() RecordVerifier {
	return &recordVerifier{}
}

type recordVerifier struct {
	configuration nodeconf.NodeConf
	networkKey    crypto.PubKey
	store         crypto.KeyStorage
}

func (r *recordVerifier) Init(a *app.App) (err error) {
	r.configuration = a.MustComponent(nodeconf.CName).(nodeconf.NodeConf)
	r.store = crypto.NewKeyStorage()
	networkId := r.configuration.Configuration().NetworkId
	r.networkKey, err = crypto.DecodeNetworkId(networkId)
	return
}

func (r *recordVerifier) Name() (name string) {
	return CName
}

func (r *recordVerifier) VerifyAcceptor(rec *consensusproto.RawRecord) (err error) {
	identity, err := r.store.PubKeyFromProto(rec.AcceptorIdentity)
	if err != nil {
		identity, err = crypto.UnmarshalEd25519PublicKey(rec.AcceptorIdentity)
		if err != nil {
			return fmt.Errorf("failed to get acceptor identity: %w", err)
		}
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
