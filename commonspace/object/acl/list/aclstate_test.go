package list

import (
	"crypto/rand"
	"github.com/anyproto/any-sync/util/crypto"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAclStateIsEmpty(t *testing.T) {
	t.Run("not empty when invites", func(t *testing.T) {
		a := NewAclExecutor("spaceId")
		cmds := []string{
			"a.init::a",
			"a.invite::invId",
		}
		for _, cmd := range cmds {
			err := a.Execute(cmd)
			require.NoError(t, err)
		}
		st := a.ActualAccounts()["a"].Acl.AclState()
		require.False(t, st.IsEmpty())
	})
	t.Run("not empty when joining requests", func(t *testing.T) {
		a := NewAclExecutor("spaceId")
		cmds := []string{
			"a.init::a",
			"a.invite::invId",
			"b.join::invId",
			"a.revoke::invId",
		}
		for _, cmd := range cmds {
			err := a.Execute(cmd)
			require.NoError(t, err)
		}
		st := a.ActualAccounts()["a"].Acl.AclState()
		require.False(t, st.IsEmpty())
	})
	t.Run("not empty when users", func(t *testing.T) {
		a := NewAclExecutor("spaceId")
		cmds := []string{
			"a.init::a",
			"a.invite::invId",
			"b.join::invId",
			"a.revoke::invId",
			"a.approve::b,r",
		}
		for _, cmd := range cmds {
			err := a.Execute(cmd)
			require.NoError(t, err)
		}
		st := a.ActualAccounts()["a"].Acl.AclState()
		require.False(t, st.IsEmpty())
	})
	t.Run("empty when no joining requests, no invites and no users", func(t *testing.T) {
		a := NewAclExecutor("spaceId")
		cmds := []string{
			"a.init::a",
			"a.invite::invId",
			"b.join::invId",
			"a.decline::b",
			"a.revoke::invId",
		}
		for _, cmd := range cmds {
			err := a.Execute(cmd)
			require.NoError(t, err)
		}
		st := a.ActualAccounts()["a"].Acl.AclState()
		require.True(t, st.IsEmpty())
	})
}

func TestAclState_FirstMetadataKey(t *testing.T) {
	t.Run("returns first metadata key successfully", func(t *testing.T) {
		// given
		privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		pubKey := privKey.GetPublic()
		readKey := crypto.NewAES()

		state := &AclState{
			readKeyChanges: []string{"recordId"},
			keys: map[string]AclKeys{
				"recordId": {
					ReadKey:         readKey,
					MetadataPrivKey: privKey,
					MetadataPubKey:  pubKey,
				},
			},
		}

		// when
		key, err := state.FirstMetadataKey()

		// then
		require.NoError(t, err)
		require.Equal(t, privKey, key)
	})

	t.Run("returns error when no read key changes", func(t *testing.T) {
		// given
		state := &AclState{readKeyChanges: []string{}}

		// when
		_, err := state.FirstMetadataKey()

		// then
		require.ErrorIs(t, err, ErrNoMetadataKey)
	})

	t.Run("returns error when first read key change is missing", func(t *testing.T) {
		// given
		state := &AclState{
			readKeyChanges: []string{"missingRecord"},
			keys:           make(map[string]AclKeys)}

		// when
		_, err := state.FirstMetadataKey()

		// then
		require.ErrorIs(t, err, ErrNoMetadataKey)
	})
}
