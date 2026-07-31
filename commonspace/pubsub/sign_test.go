package pubsub

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/pubsub/pubsubproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func newTestPublish() *pubsubproto.Publish {
	return &pubsubproto.Publish{
		SpaceId:        "space1",
		Topic:          "chat/abc/typing",
		MsgId:          testMsgId(42),
		KeyId:          "key1",
		Payload:        []byte("hello"),
		TimestampMilli: 1751800000000,
	}
}

func TestSignVerifyPublish(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)

	p := newTestPublish()
	require.NoError(t, signPublish(priv, p))

	identity, err := verifyPublish(p)
	require.NoError(t, err)
	require.True(t, identity.Equals(priv.GetPublic()))

	// the relayed flag is excluded from the signature
	p.Relayed = true
	_, err = verifyPublish(p)
	require.NoError(t, err)
}

func TestVerifyPublishTampered(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)

	fields := map[string]func(p *pubsubproto.Publish){
		"payload": func(p *pubsubproto.Publish) { p.Payload = []byte("evil") },
		"topic":   func(p *pubsubproto.Publish) { p.Topic = "acc/online/victim" },
		"spaceId": func(p *pubsubproto.Publish) { p.SpaceId = "space2" },
		"msgId":   func(p *pubsubproto.Publish) { p.MsgId = testMsgId(43) },
		"keyId":   func(p *pubsubproto.Publish) { p.KeyId = "key2" },
		"ts":      func(p *pubsubproto.Publish) { p.TimestampMilli++ },
	}
	for name, tamper := range fields {
		p := newTestPublish()
		require.NoError(t, signPublish(priv, p))
		tamper(p)
		_, err = verifyPublish(p)
		require.Error(t, err, "tampered %s must fail verification", name)
	}
}

func TestVerifyPublishForeignIdentity(t *testing.T) {
	priv1, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	_, pub2, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)

	p := newTestPublish()
	require.NoError(t, signPublish(priv1, p))
	// swap in another identity: signature no longer matches
	p.Identity, err = pub2.Marshall()
	require.NoError(t, err)
	_, err = verifyPublish(p)
	require.Error(t, err)
}

func TestSignDataUnambiguous(t *testing.T) {
	// shifting bytes between adjacent fields must change the signed data
	p1 := &pubsubproto.Publish{SpaceId: "ab", Topic: "c"}
	p2 := &pubsubproto.Publish{SpaceId: "a", Topic: "bc"}
	require.NotEqual(t, publishSignData(p1), publishSignData(p2))
}
