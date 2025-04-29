package list

import (
	"crypto/rand"
	"testing"

	"github.com/anyproto/any-sync/util/crypto"

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
		privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		pubKey := privKey.GetPublic()
		readKey := crypto.NewAES()
		state := &AclState{
			id: "recordId",
			keys: map[string]AclKeys{
				"recordId": {
					ReadKey:         readKey,
					MetadataPrivKey: privKey,
					MetadataPubKey:  pubKey,
				},
			},
		}
		key, err := state.FirstMetadataKey()
		require.NoError(t, err)
		require.Equal(t, privKey, key)
	})
	t.Run("first metadata is nil", func(t *testing.T) {
		state := &AclState{
			id: "recordId",
			keys: map[string]AclKeys{
				"recordId": {
					ReadKey: crypto.NewAES(),
				},
			},
		}
		key, err := state.FirstMetadataKey()
		require.ErrorIs(t, err, ErrNoMetadataKey)
		require.Nil(t, key)
	})
	t.Run("returns error when no read key changes", func(t *testing.T) {
		state := &AclState{}
		_, err := state.FirstMetadataKey()
		require.ErrorIs(t, err, ErrNoMetadataKey)
	})
}
