package list

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChildAclSequence_RemovalRotationRemoval(t *testing.T) {
	c := newChildAcl(t, map[string]AclPermissions{
		"admin":  AclPermissionsAdmin,
		"writer": AclPermissionsWriter,
		"reader": AclPermissionsReader,
	})

	c.removeNoRotate(t, "writer")
	require.True(t, c.state(t, "owner").RotationPending())
	first := c.rotate(t, "owner")
	require.False(t, c.state(t, "owner").RotationPending())
	c.requireReadKey(t, "writer", first, false)
	c.requireReadKey(t, "reader", first, true)

	c.removeNoRotate(t, "reader")
	for _, viewer := range []string{"owner", "admin", "reader"} {
		st := c.state(t, viewer)
		require.True(t, st.RotationPending(), viewer)
		require.Equal(t, first, st.CurrentReadKeyId(), viewer)
	}
	// the second removed account holds the key of the first rotation until the next one
	c.requireReadKey(t, "reader", first, true)

	second := c.rotate(t, "admin")
	for _, viewer := range []string{"owner", "admin", "writer", "reader"} {
		require.False(t, c.state(t, viewer).RotationPending(), viewer)
	}
	c.requireReadKey(t, "owner", second, true)
	c.requireReadKey(t, "admin", second, true)
	c.requireReadKey(t, "writer", second, false)
	c.requireReadKey(t, "reader", second, false)
}

func TestChildAclSequence_TwoRemovalsOneRotation(t *testing.T) {
	c := newChildAcl(t, map[string]AclPermissions{
		"admin":  AclPermissionsAdmin,
		"writer": AclPermissionsWriter,
		"reader": AclPermissionsReader,
	})
	readKeyId := c.state(t, "owner").CurrentReadKeyId()

	first := c.removeNoRotate(t, "writer")
	second := c.removeNoRotate(t, "reader")
	st := c.state(t, "admin")
	require.True(t, st.RotationPending())
	require.Equal(t, readKeyId, st.CurrentReadKeyId())
	for _, removal := range []string{first, second} {
		perms, err := st.PermissionsAtRecord(removal, c.pub("writer"))
		require.NoError(t, err)
		require.True(t, perms.NoPermissions())
	}

	// an account already removed cannot be removed again
	rec := c.signed(t, "parent", c.removeNoRotateContent(t, "writer"))
	require.Error(t, c.admit(t, rec, c.grant("parent", authorityRemove)))

	rotation := c.rotate(t, "owner")
	require.False(t, c.state(t, "admin").RotationPending())
	c.requireReadKey(t, "admin", rotation, true)
	c.requireReadKey(t, "writer", rotation, false)
	c.requireReadKey(t, "reader", rotation, false)
}

func TestChildAclSequence_SeveralAccountsInOneRecord(t *testing.T) {
	c := newChildAcl(t, map[string]AclPermissions{
		"admin":  AclPermissionsAdmin,
		"writer": AclPermissionsWriter,
		"reader": AclPermissionsReader,
		"stays":  AclPermissionsWriter,
	})
	removed := []string{"admin", "writer", "reader"}

	removal := c.removeNoRotate(t, removed...)
	st := c.state(t, "owner")
	require.True(t, st.RotationPending())
	for _, name := range removed {
		require.True(t, st.Permissions(c.pub(name)).NoPermissions(), name)
		require.Equal(t, StatusRemoved, c.status(st, name), name)
		perms, err := st.PermissionsAtRecord(removal, c.pub(name))
		require.NoError(t, err)
		require.True(t, perms.NoPermissions(), name)
	}
	require.Equal(t, AclPermissionsWriter, st.Permissions(c.pub("stays")))

	rotation := c.rotate(t, "owner")
	require.False(t, c.state(t, "owner").RotationPending())
	c.requireReadKey(t, "stays", rotation, true)
	for _, name := range removed {
		c.requireReadKey(t, name, rotation, false)
	}
}

// An ordinary removal rotates the read key in its own record, which ends the pending state of an
// earlier keyless removal too.
func TestChildAclSequence_KeylessThenOrdinaryRemoval(t *testing.T) {
	for _, remover := range []string{"owner", "admin"} {
		t.Run(remover, func(t *testing.T) {
			c := newChildAcl(t, map[string]AclPermissions{
				"admin":  AclPermissionsAdmin,
				"writer": AclPermissionsWriter,
				"reader": AclPermissionsReader,
			})
			c.removeNoRotate(t, "writer")
			require.True(t, c.state(t, "owner").RotationPending())

			rec, err := c.builder(t, remover).BuildAccountRemove(AccountRemovePayload{
				Identities: c.pubs("reader"),
				Change:     newTestReadKeyChangePayload(),
			})
			require.NoError(t, err)
			removal := c.add(t, rec, c.grant("parent", authorityAll))

			for _, viewer := range []string{"owner", "admin", "writer", "reader", "parent"} {
				st := c.state(t, viewer)
				require.False(t, st.RotationPending(), viewer)
				require.Equal(t, removal, st.CurrentReadKeyId(), viewer)
				require.True(t, st.Permissions(c.pub("reader")).NoPermissions(), viewer)
			}
			c.requireReadKey(t, "owner", removal, true)
			c.requireReadKey(t, "admin", removal, true)
			c.requireReadKey(t, "writer", removal, false)
			c.requireReadKey(t, "reader", removal, false)
		})
	}
}

// After the parent's Owner changes the owner, the new owner acts as the owner and the previous one in the
// role the record names.
func TestChildAclSequence_OwnersActAfterParentOwnershipChange(t *testing.T) {
	newChild := func(t *testing.T, kept AclPermissions) *childAcl {
		c := newChildAcl(t, map[string]AclPermissions{"next": AclPermissionsWriter, "reader": AclPermissionsReader})
		c.changeOwnerByParent(t, "next", kept)
		return c
	}

	t.Run("the new owner", func(t *testing.T) {
		c := newChild(t, AclPermissionsAdmin)

		options, err := c.builder(t, "next").BuildSpaceOptionsChange(nil)
		require.NoError(t, err)
		c.add(t, options, nil)

		c.addMembers(t, "next", map[string]AclPermissions{"newAdmin": AclPermissionsAdmin})
		require.Equal(t, AclPermissionsAdmin, c.state(t, "owner").Permissions(c.pub("newAdmin")))

		rotation := c.rotate(t, "next")
		c.requireReadKey(t, "next", rotation, true)
		c.requireReadKey(t, "owner", rotation, true)

		// the new owner cannot be removed without the read key, the previous one can
		rec := c.signed(t, "parent", c.removeNoRotateContent(t, "next"))
		require.Error(t, c.admit(t, rec, c.grant("parent", authorityAll)))
		c.removeNoRotate(t, "owner")

		// and hands the child on with no authorizer
		handOn, err := c.builder(t, "next").BuildOwnershipChange(OwnershipChangePayload{
			NewOwner:            c.pub("newAdmin"),
			OldOwnerPermissions: AclPermissionsWriter,
		})
		require.NoError(t, err)
		c.add(t, handOn, nil)
		st := c.state(t, "reader")
		c.requireOwner(t, st, "newAdmin")
		require.Equal(t, AclPermissionsWriter, st.Permissions(c.pub("next")))
	})

	t.Run("the previous owner kept as Admin", func(t *testing.T) {
		c := newChild(t, AclPermissionsAdmin)

		_, err := c.builder(t, "owner").BuildSpaceOptionsChange(nil)
		require.Error(t, err)
		options, err := c.builder(t, "next").BuildSpaceOptionsChange(nil)
		require.NoError(t, err)
		require.Error(t, c.admit(t, c.resigned(t, options, "owner"), nil))

		_, err = c.builder(t, "owner").BuildOwnershipChange(OwnershipChangePayload{
			NewOwner:            c.pub("reader"),
			OldOwnerPermissions: AclPermissionsWriter,
		})
		require.Error(t, err)

		_, err = c.builder(t, "owner").BuildAccountsAdd(AccountsAddPayload{Additions: []AccountAdd{{
			Identity:    c.pub("outsider"),
			Permissions: AclPermissionsAdmin,
			Metadata:    mockMetadata,
		}}})
		require.Error(t, err)

		c.addMembers(t, "owner", map[string]AclPermissions{"added": AclPermissionsWriter})
		rotation := c.rotate(t, "owner")
		c.requireReadKey(t, "next", rotation, true)
		c.requireReadKey(t, "added", rotation, true)
	})

	t.Run("the previous owner kept as Writer", func(t *testing.T) {
		c := newChild(t, AclPermissionsWriter)
		st := c.state(t, "owner")
		require.True(t, st.Permissions(c.pub("owner")).CanWrite())

		_, err := c.builder(t, "owner").BuildReadKeyChange(newTestReadKeyChangePayload())
		require.Error(t, err)
		_, err = c.builder(t, "owner").BuildAccountsAdd(AccountsAddPayload{Additions: []AccountAdd{{
			Identity:    c.pub("outsider"),
			Permissions: AclPermissionsReader,
			Metadata:    mockMetadata,
		}}})
		require.Error(t, err)

		// it reads what the new owner's key protects
		rotation := c.rotate(t, "next")
		c.requireReadKey(t, "owner", rotation, true)
	})

	t.Run("the previous owner kept as Reader", func(t *testing.T) {
		c := newChild(t, AclPermissionsReader)
		st := c.state(t, "next")
		require.False(t, st.Permissions(c.pub("owner")).CanWrite())
		perms, err := st.PermissionsAtRecord(c.head(), c.pub("owner"))
		require.NoError(t, err)
		require.False(t, perms.CanWrite())

		// it asks to leave as a member does
		rec, err := c.builder(t, "owner").BuildRequestRemove()
		require.NoError(t, err)
		c.add(t, rec, nil)
	})
}

// The removed account writes no record and has no tree-write permission at the removal and after it.
func TestChildAclSequence_RemovedAccountAfterRemoval(t *testing.T) {
	c := newChildAcl(t, map[string]AclPermissions{"admin": AclPermissionsAdmin, "writer": AclPermissionsWriter})
	before := c.head()
	removal := c.removeNoRotate(t, "admin")
	coordinator := c.grant("parent", authorityAll)

	t.Run("its records are refused", func(t *testing.T) {
		builder := c.builder(t, "admin")
		_, err := builder.BuildReadKeyChange(newTestReadKeyChangePayload())
		require.Error(t, err)
		_, err = builder.BuildRequestRemove()
		require.Error(t, err)
		_, err = builder.BuildInvite()
		require.Error(t, err)
		_, err = builder.BuildAccountsAdd(AccountsAddPayload{Additions: []AccountAdd{{
			Identity:    c.pub("outsider"),
			Permissions: AclPermissionsWriter,
			Metadata:    mockMetadata,
		}}})
		require.Error(t, err)

		ownerBuilder := c.builder(t, "owner")
		rotation, err := ownerBuilder.BuildReadKeyChange(newTestReadKeyChangePayload())
		require.NoError(t, err)
		add, err := ownerBuilder.BuildAccountsAdd(AccountsAddPayload{Additions: []AccountAdd{{
			Identity:    c.pub("outsider"),
			Permissions: AclPermissionsWriter,
			Metadata:    mockMetadata,
		}}})
		require.NoError(t, err)
		remove, err := ownerBuilder.BuildAccountRemove(AccountRemovePayload{
			Identities: c.pubs("writer"),
			Change:     newTestReadKeyChangePayload(),
		})
		require.NoError(t, err)
		require.Error(t, c.admit(t, c.resigned(t, rotation, "admin"), coordinator))
		require.Error(t, c.admit(t, c.resigned(t, add, "admin"), coordinator))
		require.Error(t, c.admit(t, c.resigned(t, remove, "admin"), coordinator))
		require.Error(t, c.admit(t, c.signed(t, "admin", c.removeNoRotateContent(t, "writer")), coordinator))
	})

	rotation := c.rotate(t, "owner")
	later := c.addMembers(t, "owner", map[string]AclPermissions{"late": AclPermissionsWriter})

	for _, viewer := range []string{"owner", "writer", "admin"} {
		st := c.state(t, viewer)
		perms, err := st.PermissionsAtRecord(before, c.pub("admin"))
		require.NoError(t, err)
		require.True(t, perms.CanWrite(), viewer)
		for _, id := range []string{removal, rotation, later} {
			perms, err := st.PermissionsAtRecord(id, c.pub("admin"))
			require.NoError(t, err)
			require.False(t, perms.CanWrite(), viewer)
			require.True(t, perms.NoPermissions(), viewer)

			perms, err = st.PermissionsAtRecord(id, c.pub("writer"))
			require.NoError(t, err)
			require.True(t, perms.CanWrite(), viewer)
		}
	}
}
