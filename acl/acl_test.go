package acl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/consensus/consensusclient"
	"github.com/anyproto/any-sync/consensus/consensusclient/mock_consensusclient"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/testutil/accounttest"
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

		expRes := list.WrapAclRecord(inv.InviteRec)
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

	fx.a.Register(fx.consCl).Register(fx.AclService).Register(&accounttest.AccountTestService{})

	require.NoError(t, fx.a.Start(ctx))
	return fx
}

type fixture struct {
	a      *app.App
	ctrl   *gomock.Controller
	consCl *mock_consensusclient.MockService
	AclService
}

func (fx *fixture) finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()
}
