package list

import (
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
