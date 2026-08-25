package innerstorage

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func TestKeyValueFromProto_KeyPeerIdBinding(t *testing.T) {
	identityKey, identityPub, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	peerKey, peerPub, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	identityProto, err := identityPub.Marshall()
	require.NoError(t, err)
	peerProto, err := peerPub.Marshall()
	require.NoError(t, err)
	inner := &spacesyncproto.StoreKeyInner{
		Peer:           peerProto,
		Identity:       identityProto,
		Value:          []byte("v"),
		TimestampMicro: 1,
		AclHeadId:      "acl",
		Key:            "k",
	}
	innerBytes, err := inner.MarshalVT()
	require.NoError(t, err)
	peerSig, err := peerKey.Sign(innerBytes)
	require.NoError(t, err)
	identitySig, err := identityKey.Sign(innerBytes)
	require.NoError(t, err)
	proto := &spacesyncproto.StoreKeyValue{
		KeyPeerId:         KeyPeerId("k", peerPub.PeerId()),
		Value:             innerBytes,
		PeerSignature:     peerSig,
		IdentitySignature: identitySig,
	}

	kv, err := KeyValueFromProto(proto, true)
	require.NoError(t, err)
	require.Equal(t, "k", kv.Key)
	require.Equal(t, peerPub.PeerId(), kv.PeerId)

	// a row planted at another peer's id must not be accepted
	proto.KeyPeerId = KeyPeerId("k", "12D3KooWSomeOtherPeer")
	_, err = KeyValueFromProto(proto, true)
	require.ErrorIs(t, err, ErrKeyPeerIdMismatch)
	_, err = KeyValueFromProto(proto, false)
	require.ErrorIs(t, err, ErrKeyPeerIdMismatch)

	// nor one whose id names a different key
	proto.KeyPeerId = KeyPeerId("other", peerPub.PeerId())
	_, err = KeyValueFromProto(proto, true)
	require.ErrorIs(t, err, ErrKeyPeerIdMismatch)
}
