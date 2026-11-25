package acl

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/listtest"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusclient"
	"github.com/anyproto/any-sync/consensus/consensusclient/mock_consensusclient"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/strkey"
)

var ctx = context.Background()

func TestAclService_AddRecord(t *testing.T) {
	ownerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId := "spaceId"
	ownerAcl, err := list.NewInMemoryDerivedAcl(spaceId, ownerKeys)
	require.NoError(t, err)
	inv, err := ownerAcl.RecordBuilder().BuildInvite()
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)

		expRes := listtest.WrapAclRecord(inv.InviteRec)
		var watcherCh = make(chan consensusclient.Watcher)
		fx.consCl.EXPECT().Watch(spaceId, gomock.Any()).DoAndReturn(func(spaceId string, w consensusclient.Watcher) error {
			go func() {
				w.AddConsensusRecords([]*consensusproto.RawRecordWithId{
					ownerAcl.Root(),
				})
				watcherCh <- w
			}()
			return nil
		})

		fx.consCl.EXPECT().AddRecord(ctx, spaceId, inv.InviteRec).Return(expRes, nil)
		fx.consCl.EXPECT().UnWatch(spaceId)

		res, err := fx.AddRecord(ctx, spaceId, inv.InviteRec, Limits{
			ReadMembers:  10,
			WriteMembers: 10,
		})
		assert.Equal(t, expRes, res)
		assert.NoError(t, err)

		w := <-watcherCh
		w.AddConsensusRecords([]*consensusproto.RawRecordWithId{
			expRes,
		})
	})
	t.Run("error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)

		var testErr = errors.New("test")

		fx.consCl.EXPECT().Watch(spaceId, gomock.Any()).DoAndReturn(func(spaceId string, w consensusclient.Watcher) error {
			go func() {
				w.AddConsensusError(testErr)
			}()
			return nil
		})
		fx.consCl.EXPECT().UnWatch(spaceId)

		res, err := fx.AddRecord(ctx, spaceId, inv.InviteRec, Limits{
			ReadMembers:  10,
			WriteMembers: 10,
		})
		assert.Nil(t, res)
		assert.EqualError(t, err, testErr.Error())
	})
	t.Run("limit exceed", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		_, err := fx.AddRecord(ctx, spaceId, inv.InviteRec, Limits{
			ReadMembers:  1,
			WriteMembers: 1,
		})
		assert.ErrorIs(t, err, ErrLimitExceed)
	})
}

// accept signs the record as the network does and gives it its id
func (fx *fixture) accept(t *testing.T, rec *consensusproto.RawRecord) *consensusproto.RawRecordWithId {
	var err error
	rec.AcceptorIdentity, err = fx.netKey.GetPublic().Marshall()
	require.NoError(t, err)
	rec.AcceptorSignature, err = fx.netKey.Sign(rec.Payload)
	require.NoError(t, err)
	return listtest.WrapAclRecord(rec)
}

// watch makes consensus serve the log, oldest record first
func (fx *fixture) watch(spaceId string, log ...*consensusproto.RawRecordWithId) {
	fx.consCl.EXPECT().Watch(spaceId, gomock.Any()).DoAndReturn(func(spaceId string, w consensusclient.Watcher) error {
		recs := slices.Clone(log)
		slices.Reverse(recs)
		go w.AddConsensusRecords(recs)
		return nil
	})
	fx.consCl.EXPECT().UnWatch(spaceId)
}

func TestAclService_AddRecordOptions(t *testing.T) {
	spaceId := "childSpaceId"
	limits := Limits{ReadMembers: 10, WriteMembers: 10}

	// the acl of a child space with its owner and one more member, and an account of the parent space
	// that is not a member of the child
	type child struct {
		*fixture
		log     []*consensusproto.RawRecordWithId
		member  crypto.PubKey
		parent  crypto.PubKey
		removal *consensusproto.RawRecord
		addOne  *consensusproto.RawRecord
	}
	newChild := func(t *testing.T) *child {
		fx := newFixture(t)
		t.Cleanup(func() { fx.finish(t) })
		ownerKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		memberKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		parentKeys, err := accountdata.NewRandom()
		require.NoError(t, err)
		oneMoreKeys, err := accountdata.NewRandom()
		require.NoError(t, err)

		ownerAcl, err := list.NewInMemoryDerivedAcl(spaceId, ownerKeys)
		require.NoError(t, err)
		add, err := ownerAcl.RecordBuilder().BuildAccountsAdd(list.AccountsAddPayload{Additions: []list.AccountAdd{{
			Identity:    memberKeys.SignKey.GetPublic(),
			Permissions: list.AclPermissionsWriter,
		}}})
		require.NoError(t, err)
		added := fx.accept(t, add)
		require.NoError(t, ownerAcl.AddRawRecord(added))
		log := []*consensusproto.RawRecordWithId{ownerAcl.Root(), added}

		// the parent's account builds on the records alone
		storage, err := list.NewInMemoryStorage(spaceId, log)
		require.NoError(t, err)
		parentAcl, err := list.BuildAclListWithIdentity(parentKeys, storage, recordverifier.New(fx.netKey.GetPublic()))
		require.NoError(t, err)
		removal, err := parentAcl.RecordBuilder().BuildAccountRemoveNoRotate([]crypto.PubKey{memberKeys.SignKey.GetPublic()})
		require.NoError(t, err)
		addOne, err := ownerAcl.RecordBuilder().BuildAccountsAdd(list.AccountsAddPayload{Additions: []list.AccountAdd{{
			Identity:    oneMoreKeys.SignKey.GetPublic(),
			Permissions: list.AclPermissionsWriter,
		}}})
		require.NoError(t, err)

		fx.watch(spaceId, log...)
		return &child{
			fixture: fx,
			log:     log,
			member:  memberKeys.SignKey.GetPublic(),
			parent:  parentKeys.SignKey.GetPublic(),
			removal: removal,
			addOne:  addOne,
		}
	}
	authorize := func(c *child) list.ParentAuthorizer {
		return func(author crypto.PubKey) (list.ParentAuthority, error) {
			return list.ParentAuthority{RemoveMembers: author.Equals(c.parent)}, nil
		}
	}

	t.Run("a record on the parent's authority is refused without the authorizer", func(t *testing.T) {
		c := newChild(t)
		_, err := c.AddRecord(ctx, spaceId, c.removal, limits)
		require.ErrorIs(t, err, list.ErrInsufficientPermissions)
	})

	t.Run("it is added with the authorizer", func(t *testing.T) {
		c := newChild(t)
		expRes := listtest.WrapAclRecord(c.removal)
		c.consCl.EXPECT().AddRecord(ctx, spaceId, c.removal).Return(expRes, nil)
		res, err := c.AddRecord(ctx, spaceId, c.removal, limits, WithParentAuthorizer(authorize(c)))
		require.NoError(t, err)
		require.Equal(t, expRes, res)
	})

	t.Run("the check of a valid record sees the state before and after it", func(t *testing.T) {
		c := newChild(t)
		c.consCl.EXPECT().AddRecord(ctx, spaceId, c.removal).Return(listtest.WrapAclRecord(c.removal), nil)
		var calls int
		_, err := c.AddRecord(ctx, spaceId, c.removal, limits, WithParentAuthorizer(authorize(c)),
			WithAfterValid(func(before, after *list.AclState) error {
				calls++
				require.Equal(t, list.AclPermissionsWriter, before.Permissions(c.member))
				require.False(t, before.RotationPending())
				require.True(t, after.Permissions(c.member).NoPermissions())
				require.True(t, after.RotationPending())
				return nil
			}))
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})

	t.Run("its error refuses the record", func(t *testing.T) {
		c := newChild(t)
		refusal := errors.New("refused")
		_, err := c.AddRecord(ctx, spaceId, c.removal, limits, WithParentAuthorizer(authorize(c)),
			WithAfterValid(func(before, after *list.AclState) error { return refusal }))
		require.ErrorIs(t, err, refusal)
	})

	t.Run("it does not run for a record over the limits", func(t *testing.T) {
		c := newChild(t)
		_, err := c.AddRecord(ctx, spaceId, c.addOne, Limits{ReadMembers: 2, WriteMembers: 2},
			WithAfterValid(func(before, after *list.AclState) error {
				t.Fatal("the check ran for a record over the limits")
				return nil
			}))
		require.ErrorIs(t, err, ErrLimitExceed)
	})
}

func TestAclService_RecordsAfter(t *testing.T) {
	ownerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId := "spaceId"
	ownerAcl, err := list.NewInMemoryDerivedAcl(spaceId, ownerKeys)
	require.NoError(t, err)

	fx := newFixture(t)
	defer fx.finish(t)

	fx.consCl.EXPECT().Watch(spaceId, gomock.Any()).DoAndReturn(func(spaceId string, w consensusclient.Watcher) error {
		go func() {
			w.AddConsensusRecords([]*consensusproto.RawRecordWithId{
				ownerAcl.Root(),
			})
		}()
		return nil
	})
	fx.consCl.EXPECT().UnWatch(spaceId)

	res, err := fx.RecordsAfter(ctx, spaceId, "")
	require.NoError(t, err)
	assert.Len(t, res, 1)
}

func TestAclService(t *testing.T) {
	ownerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId := "spaceId"
	ownerAcl, err := list.NewInMemoryDerivedAcl(spaceId, ownerKeys)
	require.NoError(t, err)

	fx := newFixture(t)
	defer fx.finish(t)

	fx.consCl.EXPECT().Watch(spaceId, gomock.Any()).DoAndReturn(func(spaceId string, w consensusclient.Watcher) error {
		go func() {
			w.AddConsensusRecords([]*consensusproto.RawRecordWithId{
				ownerAcl.Root(),
			})
		}()
		return nil
	})
	fx.consCl.EXPECT().UnWatch(spaceId)

	t.Run("permissions", func(t *testing.T) {
		res, err := fx.Permissions(ctx, ownerKeys.SignKey.GetPublic(), spaceId)
		require.NoError(t, err)
		assert.True(t, res.IsOwner())
	})
	t.Run("ownerPubKey", func(t *testing.T) {
		res, err := fx.OwnerPubKey(ctx, spaceId)
		require.NoError(t, err)
		assert.Equal(t, ownerKeys.SignKey.GetPublic().Account(), res.Account())
	})
}

func TestAclService_ReadList(t *testing.T) {
	ownerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId := "spaceId"
	ownerAcl, err := list.NewInMemoryDerivedAcl(spaceId, ownerKeys)
	require.NoError(t, err)

	fx := newFixture(t)
	defer fx.finish(t)

	fx.consCl.EXPECT().Watch(spaceId, gomock.Any()).DoAndReturn(func(spaceId string, w consensusclient.Watcher) error {
		go func() {
			w.AddConsensusRecords([]*consensusproto.RawRecordWithId{
				ownerAcl.Root(),
			})
		}()
		return nil
	})
	fx.consCl.EXPECT().UnWatch(spaceId)

	require.NoError(t, fx.ReadList(ctx, spaceId, func(s list.AclList) error {
		assert.NotNil(t, s)
		return nil
	}))
}

func TestAclService_HasRecord(t *testing.T) {
	ownerKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	spaceId := "spaceId"
	ownerAcl, err := list.NewInMemoryDerivedAcl(spaceId, ownerKeys)
	require.NoError(t, err)

	fx := newFixture(t)
	defer fx.finish(t)

	fx.consCl.EXPECT().Watch(spaceId, gomock.Any()).DoAndReturn(func(spaceId string, w consensusclient.Watcher) error {
		go func() {
			w.AddConsensusRecords([]*consensusproto.RawRecordWithId{
				ownerAcl.Root(),
			})
		}()
		return nil
	})
	fx.consCl.EXPECT().UnWatch(spaceId)

	has, err := fx.HasRecord(ctx, spaceId, ownerAcl.Root().Id)
	require.NoError(t, err)
	assert.True(t, has)

	has, err = fx.HasRecord(ctx, spaceId, "non-exists")
	require.NoError(t, err)
	assert.False(t, has)
}

func newFixture(t *testing.T) *fixture {
	return newFixtureOfNetwork(t, true)
}

// newFixtureOfNetwork makes the service of a network, which verifies the network's signature on the
// records it reads, or of a configuration without a network id, which validates their content in full
func newFixtureOfNetwork(t *testing.T, hasNetworkId bool) *fixture {
	ctrl := gomock.NewController(t)
	fx := &fixture{
		a:          new(app.App),
		ctrl:       ctrl,
		consCl:     mock_consensusclient.NewMockService(ctrl),
		AclService: New(),
	}

	fx.consCl.EXPECT().Name().Return(consensusclient.CName).AnyTimes()
	fx.consCl.EXPECT().Init(gomock.Any()).AnyTimes()
	fx.consCl.EXPECT().Run(gomock.Any()).AnyTimes()
	fx.consCl.EXPECT().Close(gomock.Any()).AnyTimes()

	netKey, netPub, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	fx.netKey = netKey
	netPubRaw, err := netPub.Raw()
	require.NoError(t, err)
	networkId, err := strkey.Encode(strkey.NetworkAddressVersionByte, netPubRaw)
	require.NoError(t, err)
	if !hasNetworkId {
		networkId = ""
	}

	nc := mock_nodeconf.NewMockService(ctrl)
	nc.EXPECT().Name().Return(nodeconf.CName).AnyTimes()
	nc.EXPECT().Init(gomock.Any()).AnyTimes()
	nc.EXPECT().Run(gomock.Any()).AnyTimes()
	nc.EXPECT().Close(gomock.Any()).AnyTimes()
	nc.EXPECT().Configuration().Return(nodeconf.Configuration{NetworkId: networkId}).AnyTimes()

	fx.a.Register(fx.consCl).Register(fx.AclService).Register(&accounttest.AccountTestService{}).Register(nc)

	require.NoError(t, fx.a.Start(ctx))
	return fx
}

type fixture struct {
	a      *app.App
	ctrl   *gomock.Controller
	consCl *mock_consensusclient.MockService
	netKey crypto.PrivKey
	AclService
}

func (fx *fixture) finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()
}
