package pubsub

import (
	"encoding/binary"

	"github.com/anyproto/any-sync/commonspace/pubsub/pubsubproto"
	"github.com/anyproto/any-sync/util/crypto"
)

const signPrefix = "anysync:pubsub:v1"

// publishSignData builds the byte string covered by a Publish signature.
// Every variable-length field is length-prefixed so the encoding is unambiguous
// (plain concatenation would let field boundaries shift). The relayed flag is
// excluded because nodes mutate it in transit.
func publishSignData(p *pubsubproto.Publish) []byte {
	size := len(signPrefix) + 4*4 + 8 +
		len(p.SpaceId) + len(p.Topic) + len(p.MsgId) + len(p.KeyId) + len(p.Payload)
	buf := make([]byte, 0, size)
	buf = append(buf, signPrefix...)
	for _, f := range [][]byte{[]byte(p.SpaceId), []byte(p.Topic), p.MsgId, []byte(p.KeyId)} {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(f)))
		buf = append(buf, f...)
	}
	buf = binary.LittleEndian.AppendUint64(buf, uint64(p.TimestampMilli))
	buf = append(buf, p.Payload...)
	return buf
}

// signPublish stamps identity and signature on the message using the account key.
func signPublish(key crypto.PrivKey, p *pubsubproto.Publish) (err error) {
	p.Identity, err = key.GetPublic().Marshall()
	if err != nil {
		return
	}
	p.Signature, err = key.Sign(publishSignData(p))
	return
}

// identityOf unmarshals the sender's public key from the message without verifying
// the signature. Cheap: lets the receiver run membership/ownership filters before
// the expensive Ed25519 verify, so a forged-signature flood is shed cheaply.
func identityOf(p *pubsubproto.Publish) (crypto.PubKey, error) {
	return crypto.UnmarshalEd25519PublicKeyProto(p.Identity)
}

// verifySignature checks the message signature against an already-unmarshalled key.
func verifySignature(pubKey crypto.PubKey, p *pubsubproto.Publish) error {
	ok, err := pubKey.Verify(publishSignData(p), p.Signature)
	if err != nil {
		return err
	}
	if !ok {
		return pubsubproto.ErrInvalidMessage
	}
	return nil
}

// verifyPublish unmarshals the identity and verifies the signature in one step.
func verifyPublish(p *pubsubproto.Publish) (crypto.PubKey, error) {
	pubKey, err := identityOf(p)
	if err != nil {
		return nil, err
	}
	if err = verifySignature(pubKey, p); err != nil {
		return nil, err
	}
	return pubKey, nil
}
