package list

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-sync/util/crypto"
)

// The coordinator admits an ownership change from an author who is not a member of the child only with
// the authority to change the owner.
func TestParentOwnershipChange_Admission(t *testing.T) {
	lookupErr := errors.New("the parent acl is not available")
	for _, tc := range []struct {
		name       string
		authorizer func(c *childAcl) ParentAuthorizer
		admitted   bool
	}{
		{"no authorizer", func(*childAcl) ParentAuthorizer { return nil }, false},
		{"no authority", func(c *childAcl) ParentAuthorizer { return c.grant("parent", ParentAuthority{}) }, false},
		{"authority to remove members only", func(c *childAcl) ParentAuthorizer { return c.grant("parent", authorityRemove) }, false},
		{"authority given to another account", func(c *childAcl) ParentAuthorizer { return c.grant("writer", authorityAll) }, false},
		{"the authorizer fails", func(*childAcl) ParentAuthorizer {
			return func(crypto.PubKey) (ParentAuthority, error) { return authorityAll, lookupErr }
		}, false},
		{"authority to change the owner", func(c *childAcl) ParentAuthorizer { return c.grant("parent", authorityChangeOwner) }, true},
		{"all authority", func(c *childAcl) ParentAuthorizer { return c.grant("parent", authorityAll) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newChildAcl(t, map[string]AclPermissions{"writer": AclPermissionsWriter})
			rec := c.signed(t, "parent", c.ownershipChangeContent(t, "writer", AclPermissionsAdmin))
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
		rec := c.signed(t, "parent", c.ownershipChangeContent(t, "writer", AclPermissionsAdmin))
		require.Error(t, c.build(t, "coordinator", c.networkVerifier()).ValidateRawRecord(rec, nil))
	})

	t.Run("the parent's Owner can be a member of the child", func(t *testing.T) {
		c := newChildAcl(t, map[string]AclPermissions{"admin": AclPermissionsAdmin, "writer": AclPermissionsWriter})
		rec := c.signed(t, "writer", c.ownershipChangeContent(t, "admin", AclPermissionsWriter))
		require.Error(t, c.admit(t, rec, nil))
		require.Error(t, c.admit(t, rec, c.grant("parent", authorityAll)))
		require.NoError(t, c.admit(t, rec, c.grant("writer", authorityChangeOwner)))
	})
}

// The parent's Owner hands the child to an active member of it.
func TestParentOwnershipChange_NewOwner(t *testing.T) {
	for _, tc := range []struct {
		name     string
		newOwner string
		prepare  func(t *testing.T, c *childAcl)
		admitted bool
	}{
		{name: "an Admin", newOwner: "admin", admitted: true},
		{name: "a Writer", newOwner: "writer", admitted: true},
		{name: "a Reader", newOwner: "reader", admitted: true},
		{name: "an account that was never a member", newOwner: "outsider"},
		{name: "the author, not a member", newOwner: "parent"},
		{name: "the owner", newOwner: "owner"},
		{name: "a removed member", newOwner: "writer", prepare: func(t *testing.T, c *childAcl) {
			c.removeNoRotate(t, "writer")
		}},
		{name: "a member who asked to leave", newOwner: "writer", prepare: func(t *testing.T, c *childAcl) {
			rec, err := c.builder(t, "writer").BuildRequestRemove()
			require.NoError(t, err)
			c.add(t, rec, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newChildAcl(t, map[string]AclPermissions{
				"admin":  AclPermissionsAdmin,
				"writer": AclPermissionsWriter,
				"reader": AclPermissionsReader,
			})
			if tc.prepare != nil {
				tc.prepare(t, c)
			}
			rec := c.signed(t, "parent", c.ownershipChangeContent(t, tc.newOwner, AclPermissionsAdmin))
			err := c.admit(t, rec, c.grant("parent", authorityAll))
			if tc.admitted {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// The previous owner keeps the role the record names and stays a member.
func TestParentOwnershipChange_PreviousOwner(t *testing.T) {
	for _, kept := range []AclPermissions{AclPermissionsAdmin, AclPermissionsWriter, AclPermissionsReader} {
		t.Run(aclPermissionsName(kept), func(t *testing.T) {
			c := newChildAcl(t, map[string]AclPermissions{"writer": AclPermissionsWriter, "reader": AclPermissionsReader})
			before := c.head()
			change := c.changeOwnerByParent(t, "writer", kept)

			for _, viewer := range []string{"owner", "writer", "reader", "parent", "coordinator"} {
				st := c.state(t, viewer)
				c.requireOwner(t, st, "writer")
				_, ownerRecord, err := st.OwnerPubKeyWithRecordId()
				require.NoError(t, err)
				require.Equal(t, change, ownerRecord)
				require.True(t, st.Permissions(c.pub("writer")).IsOwner())

				require.Equal(t, kept, st.Permissions(c.pub("owner")))
				require.Equal(t, StatusActive, c.status(st, "owner"))
				perms, err := st.PermissionsAtRecord(before, c.pub("owner"))
				require.NoError(t, err)
				require.True(t, perms.IsOwner())
				perms, err = st.PermissionsAtRecord(change, c.pub("owner"))
				require.NoError(t, err)
				require.Equal(t, kept, perms)

				require.Equal(t, AclPermissionsReader, st.Permissions(c.pub("reader")))
				require.True(t, st.Permissions(c.pub("parent")).NoPermissions())
				c.requireNoAccount(t, st, "parent")
			}
		})
	}

	for _, kept := range []AclPermissions{AclPermissionsOwner, AclPermissionsNone} {
		t.Run("refused "+aclPermissionsName(kept), func(t *testing.T) {
			c := newChildAcl(t, map[string]AclPermissions{"writer": AclPermissionsWriter})
			rec := c.signed(t, "parent", c.ownershipChangeContent(t, "writer", kept))
			require.Error(t, c.admit(t, rec, c.grant("parent", authorityAll)))
		})
	}
}

// The child's own Owner changes the owner without an authorizer.
func TestOwnershipChange_ByChildOwner(t *testing.T) {
	for name, authorizer := range map[string]func(c *childAcl) ParentAuthorizer{
		"no authorizer": func(*childAcl) ParentAuthorizer { return nil },
		"no authority":  func(c *childAcl) ParentAuthorizer { return c.grant("parent", ParentAuthority{}) },
	} {
		t.Run(name, func(t *testing.T) {
			c := newChildAcl(t, map[string]AclPermissions{"admin": AclPermissionsAdmin})
			rec, err := c.builder(t, "owner").BuildOwnershipChange(OwnershipChangePayload{
				NewOwner:            c.pub("admin"),
				OldOwnerPermissions: AclPermissionsWriter,
			})
			require.NoError(t, err)
			require.NoError(t, c.build(t, "coordinator", c.networkVerifier()).ValidateRawRecord(rec, nil))
			c.add(t, rec, authorizer(c))

			st := c.state(t, "owner")
			c.requireOwner(t, st, "admin")
			require.Equal(t, AclPermissionsWriter, st.Permissions(c.pub("owner")))
			require.Equal(t, StatusActive, c.status(st, "owner"))
		})
	}
}

func aclPermissionsName(p AclPermissions) string {
	switch p {
	case AclPermissionsNone:
		return "none"
	case AclPermissionsReader:
		return "reader"
	case AclPermissionsWriter:
		return "writer"
	case AclPermissionsAdmin:
		return "admin"
	case AclPermissionsOwner:
		return "owner"
	case AclPermissionsGuest:
		return "guest"
	}
	return "unknown"
}
