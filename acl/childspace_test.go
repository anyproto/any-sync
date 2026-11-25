package acl

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusclient"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

// childSpace is the acl log of a child space that the service loads: "owner" owns the child, "member"
// and "other" are Writers in it, and "parent" is an account of the parent space that is not a member.
type childSpace struct {
	*fixture
	spaceId string
	keys    map[string]*accountdata.AccountKeys
	log     []*consensusproto.RawRecordWithId
}

func newChildSpace(t *testing.T) *childSpace {
	return newChildSpaceOf(t, newFixture(t))
}

func newChildSpaceOf(t *testing.T, fx *fixture) *childSpace {
	t.Cleanup(func() { fx.finish(t) })
	c := &childSpace{fixture: fx, spaceId: "childSpaceId", keys: map[string]*accountdata.AccountKeys{}}
	for _, name := range []string{"owner", "member", "other", "parent", "outsider"} {
		keys, err := accountdata.NewRandom()
		require.NoError(t, err)
		c.keys[name] = keys
	}
	ownerAcl, err := list.NewInMemoryDerivedAcl(c.spaceId, c.keys["owner"])
	require.NoError(t, err)
	c.log = []*consensusproto.RawRecordWithId{ownerAcl.Root()}
	add, err := c.builder(t, "owner").BuildAccountsAdd(list.AccountsAddPayload{Additions: []list.AccountAdd{
		{Identity: c.pub("member"), Permissions: list.AclPermissionsWriter},
		{Identity: c.pub("other"), Permissions: list.AclPermissionsWriter},
	}})
	require.NoError(t, err)
	c.log = append(c.log, fx.accept(t, add))
	return c
}

func (c *childSpace) pub(name string) crypto.PubKey {
	return c.keys[name].SignKey.GetPublic()
}

// builder builds records as the account does, on the log the network serves
func (c *childSpace) builder(t *testing.T, name string) list.AclRecordBuilder {
	storage, err := list.NewInMemoryStorage(c.log[0].Id, c.log)
	require.NoError(t, err)
	l, err := list.BuildAclListWithIdentity(c.keys[name], storage, recordverifier.New(c.netKey.GetPublic()))
	require.NoError(t, err)
	return l.RecordBuilder()
}

func (c *childSpace) grant(name string, authority list.ParentAuthority) AddRecordOption {
	return WithParentAuthorizer(func(author crypto.PubKey) (list.ParentAuthority, error) {
		if author.Equals(c.pub(name)) {
			return authority, nil
		}
		return list.ParentAuthority{}, nil
	})
}

func (c *childSpace) removal(t *testing.T) *consensusproto.RawRecord {
	rec, err := c.builder(t, "parent").BuildAccountRemoveNoRotate([]crypto.PubKey{c.pub("other")})
	require.NoError(t, err)
	return rec
}

func (c *childSpace) ownershipChangeByParent(t *testing.T) *consensusproto.RawRecord {
	rec, err := c.builder(t, "parent").BuildOwnershipChangeByParent(list.OwnershipChangePayload{
		NewOwner:            c.pub("member"),
		OldOwnerPermissions: list.AclPermissionsAdmin,
	})
	require.NoError(t, err)
	return rec
}

func (c *childSpace) expectSent(rec *consensusproto.RawRecord) *consensusproto.RawRecordWithId {
	res := listtest.WrapAclRecord(rec)
	c.consCl.EXPECT().AddRecord(ctx, c.spaceId, rec).Return(res, nil)
	return res
}

var (
	removeMembersAuthority = list.ParentAuthority{RemoveMembers: true}
	changeOwnerAuthority   = list.ParentAuthority{ChangeOwner: true}
)

func failingAuthorizer() AddRecordOption {
	return WithParentAuthorizer(func(crypto.PubKey) (list.ParentAuthority, error) {
		return list.ParentAuthority{RemoveMembers: true, ChangeOwner: true}, errors.New("the parent acl is not available")
	})
}

// A record written on the authority of the parent is admitted only with an authorizer that gives its
// author that authority; nothing refused is sent to consensus.
func TestAclService_AddRecordOnParentAuthority(t *testing.T) {
	limits := Limits{ReadMembers: 10, WriteMembers: 10}
	ownershipChangeByOwner := func(c *childSpace, t *testing.T) *consensusproto.RawRecord {
		rec, err := c.builder(t, "owner").BuildOwnershipChange(list.OwnershipChangePayload{
			NewOwner:            c.pub("member"),
			OldOwnerPermissions: list.AclPermissionsAdmin,
		})
		require.NoError(t, err)
		return rec
	}
	for _, tc := range []struct {
		name     string
		record   func(c *childSpace, t *testing.T) *consensusproto.RawRecord
		opts     func(c *childSpace) []AddRecordOption
		admitted bool
	}{
		{"keyless removal without an authorizer", (*childSpace).removal, func(*childSpace) []AddRecordOption { return nil }, false},
		{"keyless removal with the authority to change the owner", (*childSpace).removal, func(c *childSpace) []AddRecordOption {
			return []AddRecordOption{c.grant("parent", changeOwnerAuthority)}
		}, false},
		{"keyless removal with a failing authorizer", (*childSpace).removal, func(*childSpace) []AddRecordOption {
			return []AddRecordOption{failingAuthorizer()}
		}, false},
		{"keyless removal with the authority to remove members", (*childSpace).removal, func(c *childSpace) []AddRecordOption {
			return []AddRecordOption{c.grant("parent", removeMembersAuthority)}
		}, true},
		{"ownership change by the parent without an authorizer", (*childSpace).ownershipChangeByParent, func(*childSpace) []AddRecordOption { return nil }, false},
		{"ownership change by the parent with the authority to remove members", (*childSpace).ownershipChangeByParent, func(c *childSpace) []AddRecordOption {
			return []AddRecordOption{c.grant("parent", removeMembersAuthority)}
		}, false},
		{"ownership change by the parent with a failing authorizer", (*childSpace).ownershipChangeByParent, func(*childSpace) []AddRecordOption {
			return []AddRecordOption{failingAuthorizer()}
		}, false},
		{"ownership change by the parent with the authority to change the owner", (*childSpace).ownershipChangeByParent, func(c *childSpace) []AddRecordOption {
			return []AddRecordOption{c.grant("parent", changeOwnerAuthority)}
		}, true},
		{"ownership change by the child's owner without an authorizer", ownershipChangeByOwner, func(*childSpace) []AddRecordOption { return nil }, true},
		{"ownership change by the child's owner with an authorizer", ownershipChangeByOwner, func(c *childSpace) []AddRecordOption {
			return []AddRecordOption{c.grant("parent", changeOwnerAuthority)}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newChildSpace(t)
			c.watch(c.spaceId, c.log...)
			rec := tc.record(c, t)
			if !tc.admitted {
				_, err := c.AddRecord(ctx, c.spaceId, rec, limits, tc.opts(c)...)
				require.Error(t, err)
				return
			}
			want := c.expectSent(rec)
			res, err := c.AddRecord(ctx, c.spaceId, rec, limits, tc.opts(c)...)
			require.NoError(t, err)
			require.Equal(t, want, res)
		})
	}
}

func TestAclService_AddRecordAfterValid(t *testing.T) {
	limits := Limits{ReadMembers: 10, WriteMembers: 10}

	t.Run("it sees the state before and after the parent's ownership change", func(t *testing.T) {
		c := newChildSpace(t)
		c.watch(c.spaceId, c.log...)
		rec := c.ownershipChangeByParent(t)
		c.expectSent(rec)
		head := c.log[len(c.log)-1].Id
		var calls int
		_, err := c.AddRecord(ctx, c.spaceId, rec, limits, c.grant("parent", changeOwnerAuthority),
			WithAfterValid(func(before, after *list.AclState) error {
				calls++
				owner, err := before.OwnerPubKey()
				require.NoError(t, err)
				require.True(t, owner.Equals(c.pub("owner")))
				require.Equal(t, head, before.LastRecordId())
				require.Equal(t, list.AclPermissionsWriter, before.Permissions(c.pub("member")))

				owner, err = after.OwnerPubKey()
				require.NoError(t, err)
				require.True(t, owner.Equals(c.pub("member")))
				require.NotEqual(t, head, after.LastRecordId())
				require.Equal(t, list.AclPermissionsAdmin, after.Permissions(c.pub("owner")))
				require.True(t, after.Permissions(c.pub("parent")).NoPermissions())
				return nil
			}))
		require.NoError(t, err)
		require.Equal(t, 1, calls)

		// the service's acl does not change until consensus delivers the record
		require.NoError(t, c.ReadState(ctx, c.spaceId, func(s *list.AclState) error {
			require.Equal(t, head, s.LastRecordId())
			require.True(t, s.Permissions(c.pub("owner")).IsOwner())
			return nil
		}))
	})

	t.Run("its error refuses the parent's ownership change", func(t *testing.T) {
		c := newChildSpace(t)
		c.watch(c.spaceId, c.log...)
		refusal := errors.New("refused")
		_, err := c.AddRecord(ctx, c.spaceId, c.ownershipChangeByParent(t), limits, c.grant("parent", changeOwnerAuthority),
			WithAfterValid(func(before, after *list.AclState) error { return refusal }))
		require.ErrorIs(t, err, refusal)
	})

	t.Run("it is not asked about a record the authority refuses", func(t *testing.T) {
		c := newChildSpace(t)
		c.watch(c.spaceId, c.log...)
		for _, opt := range []AddRecordOption{c.grant("parent", list.ParentAuthority{}), failingAuthorizer()} {
			_, err := c.AddRecord(ctx, c.spaceId, c.removal(t), limits, opt,
				WithAfterValid(func(before, after *list.AclState) error {
					t.Fatal("the check ran for a refused record")
					return nil
				}))
			require.Error(t, err)
		}
	})

	t.Run("the limits check runs with both options", func(t *testing.T) {
		c := newChildSpace(t)
		c.watch(c.spaceId, c.log...)
		add, err := c.builder(t, "owner").BuildAccountsAdd(list.AccountsAddPayload{Additions: []list.AccountAdd{
			{Identity: c.pub("outsider"), Permissions: list.AclPermissionsWriter},
		}})
		require.NoError(t, err)
		// the child has three members
		_, err = c.AddRecord(ctx, c.spaceId, add, Limits{ReadMembers: 3, WriteMembers: 3},
			c.grant("parent", list.ParentAuthority{RemoveMembers: true, ChangeOwner: true}),
			WithAfterValid(func(before, after *list.AclState) error {
				t.Fatal("the check ran for a record over the limits")
				return nil
			}))
		require.ErrorIs(t, err, ErrLimitExceed)
	})

	t.Run("a removal is admitted in a child over its limits", func(t *testing.T) {
		c := newChildSpace(t)
		c.watch(c.spaceId, c.log...)
		rec := c.removal(t)
		c.expectSent(rec)
		_, err := c.AddRecord(ctx, c.spaceId, rec, Limits{ReadMembers: 2, WriteMembers: 2}, c.grant("parent", removeMembersAuthority))
		require.NoError(t, err)
	})
}

// The service loads a log that holds records written on the authority of the parent, and ingests more
// of them as consensus delivers them, without an authorizer.
func TestAclService_ChildLogWithParentRecords(t *testing.T) {
	for name, hasNetworkId := range map[string]bool{"on a network": true, "without a network id": false} {
		t.Run(name, func(t *testing.T) {
			testChildLogWithParentRecords(t, newChildSpaceOf(t, newFixtureOfNetwork(t, hasNetworkId)))
		})
	}
}

func testChildLogWithParentRecords(t *testing.T, c *childSpace) {
	c.log = append(c.log, c.accept(t, c.ownershipChangeByParent(t)))

	watchers := make(chan consensusclient.Watcher, 1)
	c.consCl.EXPECT().Watch(c.spaceId, gomock.Any()).DoAndReturn(func(spaceId string, w consensusclient.Watcher) error {
		recs := slices.Clone(c.log)
		slices.Reverse(recs)
		go func() {
			w.AddConsensusRecords(recs)
			watchers <- w
		}()
		return nil
	})
	c.consCl.EXPECT().UnWatch(c.spaceId)

	owner, err := c.OwnerPubKey(ctx, c.spaceId)
	require.NoError(t, err)
	require.True(t, owner.Equals(c.pub("member")))
	perms, err := c.Permissions(ctx, c.pub("owner"), c.spaceId)
	require.NoError(t, err)
	require.Equal(t, list.AclPermissionsAdmin, perms)
	watcher := <-watchers

	removal := c.accept(t, c.removal(t))
	c.log = append(c.log, removal)
	watcher.AddConsensusRecords([]*consensusproto.RawRecordWithId{removal})
	require.NoError(t, c.ReadState(ctx, c.spaceId, func(s *list.AclState) error {
		require.Equal(t, removal.Id, s.LastRecordId())
		require.True(t, s.Permissions(c.pub("other")).NoPermissions())
		require.True(t, s.RotationPending())
		return nil
	}))

	rotation, err := c.builder(t, "member").BuildReadKeyChange(list.ReadKeyChangePayload{
		MetadataKey: newMetadataKey(t),
		ReadKey:     crypto.NewAES(),
	})
	require.NoError(t, err)
	rotationWithId := c.accept(t, rotation)
	watcher.AddConsensusRecords([]*consensusproto.RawRecordWithId{rotationWithId})
	require.NoError(t, c.ReadState(ctx, c.spaceId, func(s *list.AclState) error {
		require.Equal(t, rotationWithId.Id, s.LastRecordId())
		require.False(t, s.RotationPending())
		return nil
	}))
}

func newMetadataKey(t *testing.T) crypto.PrivKey {
	key, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	return key
}
