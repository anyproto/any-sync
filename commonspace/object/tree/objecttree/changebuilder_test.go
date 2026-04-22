package objecttree

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/crypto"
)

// TestChangeBuilderBuild_EncryptionContract locks in the invariant that Build
// gates encryption on BuilderContent.Unencrypted (inverted: zero value =
// encrypted) rather than inferring it from ReadKey != nil. See Cure53
// ATY-01-003 for context.
func TestChangeBuilderBuild_EncryptionContract(t *testing.T) {
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)

	readKey, err := crypto.NewRandomAES()
	require.NoError(t, err)

	basePayload := BuilderContent{
		TreeHeadIds:    []string{"head"},
		AclHeadId:      "aclHead",
		SnapshotBaseId: "root",
		PrivKey:        keys.SignKey,
		Content:        []byte("secret payload"),
		Timestamp:      1,
	}

	t.Run("encrypted (default) without read key fails loudly", func(t *testing.T) {
		builder := NewChangeBuilder(newMockKeyStorage(), nil)
		payload := basePayload
		payload.Unencrypted = false
		payload.ReadKey = nil

		_, _, err := builder.Build(payload)
		require.ErrorIs(t, err, ErrMissingEncryptKey)
	})

	t.Run("unencrypted opt-out writes content as-is", func(t *testing.T) {
		builder := NewChangeBuilder(newMockKeyStorage(), nil)
		payload := basePayload
		payload.Unencrypted = true
		payload.ReadKey = nil

		ch, _, err := builder.Build(payload)
		require.NoError(t, err)
		require.Equal(t, payload.Content, ch.Data)
	})

	t.Run("encrypted (default) with read key produces ciphertext", func(t *testing.T) {
		builder := NewChangeBuilder(newMockKeyStorage(), nil)
		payload := basePayload
		payload.Unencrypted = false
		payload.ReadKey = readKey

		ch, _, err := builder.Build(payload)
		require.NoError(t, err)
		require.NotEqual(t, payload.Content, ch.Data, "ciphertext must not equal plaintext")

		decrypted, err := readKey.Decrypt(ch.Data)
		require.NoError(t, err)
		require.Equal(t, payload.Content, decrypted)
	})
}

// TestChangeBuilderBuild_ReadKeyIgnoredWhenUnencrypted guarantees ReadKey
// alone cannot flip the change into encrypted mode when Unencrypted is set -
// the flag is the gate, not the key's presence.
func TestChangeBuilderBuild_ReadKeyIgnoredWhenUnencrypted(t *testing.T) {
	keys, err := accountdata.NewRandom()
	require.NoError(t, err)
	readKey, err := crypto.NewRandomAES()
	require.NoError(t, err)

	builder := NewChangeBuilder(newMockKeyStorage(), nil)
	payload := BuilderContent{
		TreeHeadIds:    []string{"head"},
		AclHeadId:      "aclHead",
		SnapshotBaseId: "root",
		PrivKey:        keys.SignKey,
		Content:        []byte("plaintext"),
		Timestamp:      1,
		Unencrypted:    true,
		ReadKey:        readKey,
	}

	ch, raw, err := builder.Build(payload)
	require.NoError(t, err)
	require.Equal(t, payload.Content, ch.Data)

	// Round-trip: marshalled payload carries plaintext, not ciphertext.
	rawTree := &treechangeproto.RawTreeChange{}
	require.NoError(t, rawTree.UnmarshalVT(raw.RawChange))
	treeCh := &treechangeproto.TreeChange{}
	require.NoError(t, treeCh.UnmarshalVT(rawTree.Payload))
	require.Equal(t, payload.Content, treeCh.ChangesData)
}
