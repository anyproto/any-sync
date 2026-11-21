package list

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/util/crypto"
)

// The coordinator admits a keyless removal only from an author the parent space lets remove members.
func TestKeylessRemoval_Admission(t *testing.T) {
	lookupErr := errors.New("the parent acl is not available")
	for _, tc := range []struct {
		name       string
		authorizer func(c *childAcl) ParentAuthorizer
		admitted   bool
	}{
		{"no authorizer", func(*childAcl) ParentAuthorizer { return nil }, false},
		{"no authority", func(c *childAcl) ParentAuthorizer { return c.grant("parent", ParentAuthority{}) }, false},
		{"authority to change the owner only", func(c *childAcl) ParentAuthorizer { return c.grant("parent", authorityChangeOwner) }, false},
		{"authority given to another account", func(c *childAcl) ParentAuthorizer { return c.grant("owner", authorityAll) }, false},
		{"the authorizer fails", func(*childAcl) ParentAuthorizer {
			return func(crypto.PubKey) (ParentAuthority, error) { return authorityAll, lookupErr }
		}, false},
		{"authority to remove members", func(c *childAcl) ParentAuthorizer { return c.grant("parent", authorityRemove) }, true},
		{"all authority", func(c *childAcl) ParentAuthorizer { return c.grant("parent", authorityAll) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newChildAcl(t, map[string]AclPermissions{"writer": AclPermissionsWriter})
			rec := c.signed(t, "parent", c.removeNoRotateContent(t, "writer"))
			err := c.admit(t, rec, tc.authorizer(c))
			if tc.admitted {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}

	t.Run("validation without an authorizer never admits it", func(t *testing.T) {
		c := newChildAcl(t, map[string]AclPermissions{"writer": AclPermissionsWriter})
		rec := c.signed(t, "parent", c.removeNoRotateContent(t, "writer"))
		require.Error(t, c.build(t, "coordinator", c.networkVerifier()).ValidateRawRecord(rec, nil))
		require.NoError(t, c.admit(t, rec, c.grant("parent", authorityRemove)))
	})

	t.Run("the authorizer is asked about the author", func(t *testing.T) {
		c := newChildAcl(t, map[string]AclPermissions{"writer": AclPermissionsWriter})
		rec := c.signed(t, "parent", c.removeNoRotateContent(t, "writer"))
		var asked []crypto.PubKey
		require.NoError(t, c.admit(t, rec, func(author crypto.PubKey) (ParentAuthority, error) {
			asked = append(asked, author)
			return authorityRemove, nil
		}))
		require.NotEmpty(t, asked)
		for _, author := range asked {
			require.True(t, author.Equals(c.pub("parent")))
		}
	})

	t.Run("a member of the child needs the authority as well", func(t *testing.T) {
		c := newChildAcl(t, map[string]AclPermissions{"admin": AclPermissionsAdmin, "writer": AclPermissionsWriter})
		for _, author := range []string{"owner", "admin"} {
			rec := c.signed(t, author, c.removeNoRotateContent(t, "writer"))
			require.Error(t, c.admit(t, rec, nil), author)
			require.Error(t, c.admit(t, rec, c.grant("parent", authorityAll)), author)
			// an Admin or the Owner of the parent can also be a member of the child
			require.NoError(t, c.admit(t, rec, c.grant(author, authorityRemove)), author)
		}
	})
}

// Any member but the owner of the child can be removed without the read key.
func TestKeylessRemoval_Targets(t *testing.T) {
	for _, tc := range []struct {
		name     string
		targets  []string
		admitted bool
	}{
		{"an Admin", []string{"admin"}, true},
		{"a Writer", []string{"writer"}, true},
		{"a Reader", []string{"reader"}, true},
		{"every member but the owner", []string{"admin", "writer", "reader"}, true},
		{"the owner", []string{"owner"}, false},
		{"the owner among other members", []string{"writer", "owner"}, false},
		{"an account that is not a member", []string{"outsider"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newChildAcl(t, map[string]AclPermissions{
				"admin":  AclPermissionsAdmin,
				"writer": AclPermissionsWriter,
				"reader": AclPermissionsReader,
			})
			rec := c.signed(t, "parent", c.removeNoRotateContent(t, tc.targets...))
			err := c.admit(t, rec, c.grant("parent", authorityAll))
			if tc.admitted {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// The removal takes effect in the record: the removed account has no permissions, the read key stays
// the one it holds, and the space is rotation pending.
func TestKeylessRemoval_TakesEffectAtOnce(t *testing.T) {
	c := newChildAcl(t, map[string]AclPermissions{
		"admin":  AclPermissionsAdmin,
		"writer": AclPermissionsWriter,
		"reader": AclPermissionsReader,
	})
	before := c.head()
	readKeyId := c.state(t, "owner").CurrentReadKeyId()
	require.False(t, c.state(t, "owner").RotationPending())

	removal := c.removeNoRotate(t, "writer")

	for _, viewer := range []string{"owner", "admin", "writer", "reader", "parent", "coordinator"} {
		t.Run(viewer, func(t *testing.T) {
			st := c.state(t, viewer)
			require.Equal(t, removal, st.LastRecordId())

			require.True(t, st.Permissions(c.pub("writer")).NoPermissions())
			require.Equal(t, StatusRemoved, c.status(st, "writer"))
			perms, err := st.PermissionsAtRecord(before, c.pub("writer"))
			require.NoError(t, err)
			require.Equal(t, AclPermissionsWriter, perms)
			perms, err = st.PermissionsAtRecord(removal, c.pub("writer"))
			require.NoError(t, err)
			require.False(t, perms.CanWrite())
			require.True(t, perms.NoPermissions())

			require.True(t, st.RotationPending())
			require.Equal(t, readKeyId, st.CurrentReadKeyId())

			c.requireOwner(t, st, "owner")
			require.Equal(t, AclPermissionsAdmin, st.Permissions(c.pub("admin")))
			require.Equal(t, AclPermissionsReader, st.Permissions(c.pub("reader")))
			require.True(t, st.Permissions(c.pub("parent")).NoPermissions())
			c.requireNoAccount(t, st, "parent")
		})
	}

	// until the rotation the removed account still holds the current read key
	c.requireReadKey(t, "writer", readKeyId, true)
}

// Only an Admin or the Owner of the child writes the read key change that ends the pending state, and
// the accounts removed before it cannot decrypt the new key.
func TestKeylessRemoval_Rotation(t *testing.T) {
	members := map[string]AclPermissions{
		"admin":   AclPermissionsAdmin,
		"writer":  AclPermissionsWriter,
		"reader":  AclPermissionsReader,
		"removed": AclPermissionsAdmin,
	}
	for _, tc := range []struct {
		rotator  string
		admitted bool
	}{
		{"owner", true},
		{"admin", true},
		{"writer", false},
		{"reader", false},
		// not a member of the child, whatever its authority in the parent
		{"parent", false},
		// holds the current read key, but is no longer a member
		{"removed", false},
	} {
		t.Run(tc.rotator, func(t *testing.T) {
			c := newChildAcl(t, members)
			oldKeyId := c.state(t, "owner").CurrentReadKeyId()
			c.removeNoRotate(t, "removed")
			coordinator := c.grant("parent", authorityAll)

			if !tc.admitted {
				_, err := c.builder(t, tc.rotator).BuildReadKeyChange(newTestReadKeyChangePayload())
				require.Error(t, err)
				ownerRotation, err := c.builder(t, "owner").BuildReadKeyChange(newTestReadKeyChangePayload())
				require.NoError(t, err)
				rec := c.resigned(t, ownerRotation, tc.rotator)
				require.Error(t, c.admit(t, rec, coordinator))
				require.Error(t, c.admit(t, rec, c.grant(tc.rotator, authorityAll)))
				require.True(t, c.state(t, "owner").RotationPending())
				return
			}

			rec, err := c.builder(t, tc.rotator).BuildReadKeyChange(newTestReadKeyChangePayload())
			require.NoError(t, err)
			rotation := c.add(t, rec, coordinator)

			for _, viewer := range []string{"owner", "admin", "writer", "reader", "removed", "parent"} {
				st := c.state(t, viewer)
				require.False(t, st.RotationPending(), viewer)
				require.Equal(t, rotation, st.CurrentReadKeyId(), viewer)
			}
			newKey := c.state(t, "owner").Keys()[rotation].ReadKey
			require.NotNil(t, newKey)
			for _, member := range []string{"admin", "writer", "reader"} {
				require.True(t, newKey.Equals(c.state(t, member).Keys()[rotation].ReadKey), member)
			}
			c.requireReadKey(t, "removed", rotation, false)
			c.requireReadKey(t, "removed", oldKeyId, true)
		})
	}
}

// Records other than a read key change leave the space rotation pending.
func TestKeylessRemoval_PendingUntilReadKeyChange(t *testing.T) {
	c := newChildAcl(t, map[string]AclPermissions{
		"admin":  AclPermissionsAdmin,
		"writer": AclPermissionsWriter,
		"reader": AclPermissionsReader,
	})
	c.removeNoRotate(t, "writer")
	requirePending := func(pending bool) {
		t.Helper()
		for _, viewer := range []string{"owner", "admin", "parent"} {
			require.Equal(t, pending, c.state(t, viewer).RotationPending(), viewer)
		}
	}
	requirePending(true)

	c.addMembers(t, "admin", map[string]AclPermissions{"late": AclPermissionsWriter})
	requirePending(true)

	change, err := c.builder(t, "owner").BuildPermissionChange(PermissionChangePayload{
		Identity:    c.pub("reader"),
		Permissions: AclPermissionsWriter,
	})
	require.NoError(t, err)
	c.add(t, change, nil)
	requirePending(true)

	invite, err := c.builder(t, "admin").BuildInvite()
	require.NoError(t, err)
	c.add(t, invite.InviteRec, nil)
	requirePending(true)

	c.rotate(t, "admin")
	requirePending(false)
}

// Ordinary records of the child's members are admitted whatever the parent lets their authors do.
func TestChildAcl_MemberRecordsNeedNoParentAuthority(t *testing.T) {
	c := newChildAcl(t, map[string]AclPermissions{"admin": AclPermissionsAdmin, "writer": AclPermissionsWriter})
	for name, authorizer := range map[string]ParentAuthorizer{
		"no authorizer":    nil,
		"no authority":     c.grant("parent", ParentAuthority{}),
		"parent authority": c.grant("parent", authorityAll),
	} {
		t.Run(name, func(t *testing.T) {
			add, err := c.builder(t, "admin").BuildAccountsAdd(AccountsAddPayload{Additions: []AccountAdd{{
				Identity:    c.pub("outsider"),
				Permissions: AclPermissionsReader,
				Metadata:    mockMetadata,
			}}})
			require.NoError(t, err)
			require.NoError(t, c.admit(t, add, authorizer))

			rotation, err := c.builder(t, "owner").BuildReadKeyChange(newTestReadKeyChangePayload())
			require.NoError(t, err)
			require.NoError(t, c.admit(t, rotation, authorizer))

			ownership, err := c.builder(t, "owner").BuildOwnershipChange(OwnershipChangePayload{
				NewOwner:            c.pub("admin"),
				OldOwnerPermissions: AclPermissionsWriter,
			})
			require.NoError(t, err)
			require.NoError(t, c.admit(t, ownership, authorizer))
		})
	}
}
