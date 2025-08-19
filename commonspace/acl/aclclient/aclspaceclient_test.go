package aclclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/node/nodeclient"
	"github.com/anyproto/any-sync/node/nodeclient/mock_nodeclient"
	"github.com/anyproto/any-sync/testutil/anymock"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
)

var ctx = context.Background()

func TestAclSpaceClient_StopSharing(t *testing.T) {
	t.Run("not empty", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		cmds := []string{
			"a.invite::invId",
			"b.join::invId",
			"c.join::invId",
			"a.approve::c,r",
		}
		for _, cmd := range cmds {
			err := fx.exec.Execute(cmd)
			require.NoError(t, err)
		}
		newPrivKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		st := fx.acl.AclState()
		require.False(t, st.IsEmpty())
		fx.nodeClient.EXPECT().AclAddRecord(ctx, fx.spaceState.SpaceId, gomock.Any()).DoAndReturn(
			func(ctx context.Context, spaceId string, rec *consensusproto.RawRecord) (*consensusproto.RawRecordWithId, error) {
				return marshallRecord(t, rec), nil
			})
		err = fx.StopSharing(ctx, list.ReadKeyChangePayload{
			MetadataKey: newPrivKey,
			ReadKey:     crypto.NewAES(),
		})
		st = fx.acl.AclState()
		require.True(t, st.IsEmpty())
	})
	t.Run("already empty", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		newPrivKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		st := fx.acl.AclState()
		require.True(t, st.IsEmpty())
		err = fx.StopSharing(ctx, list.ReadKeyChangePayload{
			MetadataKey: newPrivKey,
			ReadKey:     crypto.NewAES(),
		})
		st = fx.acl.AclState()
		require.True(t, st.IsEmpty())
	})
}

type namedAcl struct {
	list.AclList
}

func (a *namedAcl) Name() string {
	return syncacl.CName
}

func (a *namedAcl) Init(app *app.App) error {
	return nil
}

func marshallRecord(t *testing.T, rec *consensusproto.RawRecord) *consensusproto.RawRecordWithId {
	data, err := rec.MarshalVT()
	require.NoError(t, err)
	recId, err := cidutil.NewCidFromBytes(data)
	require.NoError(t, err)
	return &consensusproto.RawRecordWithId{
		Payload: data,
		Id:      recId,
	}
}

type fixture struct {
	*aclSpaceClient
	a          *app.App
	ctrl       *gomock.Controller
	nodeClient *mock_nodeclient.MockNodeClient
	acl        list.AclList
	spaceState *spacestate.SpaceState
	exec       *list.AclTestExecutor
}

func newFixture(t *testing.T) *fixture {
	ctrl := gomock.NewController(t)
	spaceId := "spaceId"
	exec := list.NewAclExecutor(spaceId)
	err := exec.Execute("a.init::a")
	require.NoError(t, err)
	acl := exec.ActualAccounts()["a"].Acl
	fx := &fixture{
		aclSpaceClient: NewAclSpaceClient().(*aclSpaceClient),
		ctrl:           ctrl,
		acl:            acl,
		nodeClient:     mock_nodeclient.NewMockNodeClient(ctrl),
		spaceState:     &spacestate.SpaceState{SpaceId: spaceId},
		a:              new(app.App),
		exec:           exec,
	}
	anymock.ExpectComp(fx.nodeClient.EXPECT(), nodeclient.CName)
	fx.a.Register(fx.spaceState).
		Register(fx.nodeClient).
		Register(&namedAcl{acl}).
		Register(fx)

	require.NoError(t, fx.a.Start(ctx))
	return fx
}

func (fx *fixture) finish(t *testing.T) {
	require.NoError(t, fx.a.Close(ctx))
	fx.ctrl.Finish()
}

func TestAclSpaceClient_ChangeInvitePermissions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		
		err := fx.exec.Execute("a.invite_anyone::inviteId,r")
		require.NoError(t, err)
		
		invites := fx.acl.AclState().Invites()
		require.NotEmpty(t, invites)
		inviteId := invites[0].Id
		
		fx.nodeClient.EXPECT().AclAddRecord(ctx, fx.spaceState.SpaceId, gomock.Any()).DoAndReturn(
			func(ctx context.Context, spaceId string, rec *consensusproto.RawRecord) (*consensusproto.RawRecordWithId, error) {
				return marshallRecord(t, rec), nil
			})
		
		newPerms := list.AclPermissionsWriter
		err = fx.ChangeInvitePermissions(ctx, inviteId, newPerms)
		require.NoError(t, err)
	})
	
	t.Run("invalid invite id", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		
		newPerms := list.AclPermissionsWriter
		err := fx.ChangeInvitePermissions(ctx, "nonExistentInvite", newPerms)
		require.Error(t, err)
	})
}

func TestAclSpaceClient_ReplaceInvite(t *testing.T) {
	t.Run("replace with anyone can join invite", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		
		err := fx.exec.Execute("a.invite::inviteId1")
		require.NoError(t, err)
		
		payload := InvitePayload{
			InviteType:  aclrecordproto.AclInviteType_AnyoneCanJoin,
			Permissions: list.AclPermissionsWriter,
		}
		
		result, err := fx.ReplaceInvite(ctx, payload)
		require.NoError(t, err)
		require.NotNil(t, result.InviteRec)
		require.NotNil(t, result.InviteKey)
	})
	
	t.Run("replace with request to join invite", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		
		err := fx.exec.Execute("a.invite_anyone::inviteId1,rw")
		require.NoError(t, err)
		
		payload := InvitePayload{
			InviteType:  aclrecordproto.AclInviteType_RequestToJoin,
			Permissions: list.AclPermissionsNone,
		}
		
		result, err := fx.ReplaceInvite(ctx, payload)
		require.NoError(t, err)
		require.NotNil(t, result.InviteRec)
		require.NotNil(t, result.InviteKey)
	})
	
	t.Run("duplicate invite error", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		
		err := fx.exec.Execute("a.invite::inviteId1")
		require.NoError(t, err)
		
		state := fx.acl.AclState()
		invites := state.Invites()
		require.NotEmpty(t, invites)
		
		payload := InvitePayload{
			InviteType:  invites[0].Type,
			Permissions: invites[0].Permissions,
		}
		
		_, err = fx.ReplaceInvite(ctx, payload)
		require.ErrorIs(t, err, list.ErrDuplicateInvites)
	})
	
	t.Run("empty state", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		
		payload := InvitePayload{
			InviteType:  aclrecordproto.AclInviteType_AnyoneCanJoin,
			Permissions: list.AclPermissionsReader,
		}
		
		result, err := fx.ReplaceInvite(ctx, payload)
		require.NoError(t, err)
		require.NotNil(t, result.InviteRec)
		require.NotNil(t, result.InviteKey)
	})
	
	t.Run("multiple invites replaced and canceled", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish(t)
		
		err := fx.exec.Execute("a.invite_anyone::invite1,r")
		require.NoError(t, err)
		err = fx.exec.Execute("a.invite_anyone::invite2,rw")
		require.NoError(t, err)
		err = fx.exec.Execute("a.invite::invite3")
		require.NoError(t, err)
		
		initialInvites := fx.acl.AclState().Invites()
		require.Len(t, initialInvites, 3)
		initialInviteIds := make([]string, len(initialInvites))
		for i, inv := range initialInvites {
			initialInviteIds[i] = inv.Id
		}
		
		fx.nodeClient.EXPECT().AclAddRecord(ctx, fx.spaceState.SpaceId, gomock.Any()).DoAndReturn(
			func(ctx context.Context, spaceId string, rec *consensusproto.RawRecord) (*consensusproto.RawRecordWithId, error) {
				return marshallRecord(t, rec), nil
			})
		
		payload := InvitePayload{
			InviteType:  aclrecordproto.AclInviteType_AnyoneCanJoin,
			Permissions: list.AclPermissionsAdmin,
		}
		
		result, err := fx.ReplaceInvite(ctx, payload)
		require.NoError(t, err)
		require.NotNil(t, result.InviteRec)
		require.NotNil(t, result.InviteKey)
		
		err = fx.AddRecord(ctx, result.InviteRec)
		require.NoError(t, err)
		
		finalInvites := fx.acl.AclState().Invites()
		require.Len(t, finalInvites, 1)
		
		newInviteId := finalInvites[0].Id
		for _, oldId := range initialInviteIds {
			require.NotEqual(t, oldId, newInviteId)
		}
		
		require.Equal(t, aclrecordproto.AclInviteType_AnyoneCanJoin, finalInvites[0].Type)
		require.Equal(t, list.AclPermissionsAdmin, finalInvites[0].Permissions)
	})
}
