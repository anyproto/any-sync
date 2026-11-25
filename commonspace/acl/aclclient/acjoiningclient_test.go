package aclclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/node/nodeclient"
	"github.com/anyproto/any-sync/node/nodeclient/mock_nodeclient"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/testutil/anymock"
	"github.com/anyproto/any-sync/util/crypto"
)

// parentFixture is the client of an account of the parent space. The account is not a member of the
// child: all it has of the child is the acl the node serves.
type parentFixture struct {
	*aclJoiningClient
	a          *app.App
	nodeClient *mock_nodeclient.MockNodeClient
	parentKeys *accountdata.AccountKeys
	// the child's acl as the coordinator holds it
	childAcl list.AclList
	owner    crypto.PubKey
	member   crypto.PubKey
	spaceId  string
}

func newParentFixture(t *testing.T) *parentFixture {
	ctrl := gomock.NewController(t)
	spaceId := "childSpaceId"
	exec := list.NewAclExecutor(spaceId)
	for _, cmd := range []string{"a.init::a", "a.invite::invId", "b.join::invId", "a.approve::b,rw"} {
		require.NoError(t, exec.Execute(cmd))
	}
	parentKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	// the coordinator's list ingests the log it admitted
	owner := exec.ActualAccounts()["a"]
	log, err := owner.Acl.RecordsAfter(ctx, "")
	require.NoError(t, err)
	storage, err := list.NewInMemoryStorage(log[0].Id, log)
	require.NoError(t, err)
	childAcl, err := list.BuildAclListWithIdentity(owner.Keys, storage, recordverifier.NewValidateAcceptedLog())
	require.NoError(t, err)

	fx := &parentFixture{
		aclJoiningClient: NewAclJoiningClient().(*aclJoiningClient),
		a:                new(app.App),
		nodeClient:       mock_nodeclient.NewMockNodeClient(ctrl),
		parentKeys:       parentKeys,
		childAcl:         childAcl,
		owner:            owner.Keys.SignKey.GetPublic(),
		member:           exec.ActualAccounts()["b"].Keys.SignKey.GetPublic(),
		spaceId:          spaceId,
	}
	nodeConf := mock_nodeconf.NewMockService(ctrl)
	anymock.ExpectComp(nodeConf.EXPECT(), nodeconf.CName)
	nodeConf.EXPECT().Configuration().Return(nodeconf.Configuration{}).AnyTimes()
	anymock.ExpectComp(fx.nodeClient.EXPECT(), nodeclient.CName)
	fx.a.Register(accounttest.NewWithAcc(parentKeys)).
		Register(nodeConf).
		Register(fx.nodeClient).
		Register(fx.aclJoiningClient)
	require.NoError(t, fx.a.Start(ctx))
	t.Cleanup(func() { require.NoError(t, fx.a.Close(ctx)) })

	fx.nodeClient.EXPECT().AclGetRecords(ctx, spaceId, "").DoAndReturn(
		func(ctx context.Context, _, _ string) ([]*consensusproto.RawRecordWithId, error) {
			return fx.childAcl.RecordsAfter(ctx, "")
		}).AnyTimes()
	return fx
}

// expectAdd makes the node accept a record as the coordinator does: it validates the record with what
// the parent space lets parentKeys do, and adds it to the child's acl.
func (fx *parentFixture) expectAdd(t *testing.T, authority list.ParentAuthority) {
	authorizer := func(author crypto.PubKey) (list.ParentAuthority, error) {
		if author.Equals(fx.parentKeys.SignKey.GetPublic()) {
			return authority, nil
		}
		return list.ParentAuthority{}, nil
	}
	fx.nodeClient.EXPECT().AclAddRecord(ctx, fx.spaceId, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, rec *consensusproto.RawRecord) (*consensusproto.RawRecordWithId, error) {
			if err := fx.childAcl.ValidateRawRecordWithAuthorizer(rec, authorizer, nil); err != nil {
				return nil, err
			}
			res := marshallRecord(t, rec)
			return res, fx.childAcl.AddRawRecord(res)
		})
}

func TestAclJoiningClient_RemoveChildMembers(t *testing.T) {
	t.Run("an account of the parent removes a member of the child", func(t *testing.T) {
		fx := newParentFixture(t)
		fx.expectAdd(t, list.ParentAuthority{RemoveMembers: true})

		headId, err := fx.RemoveChildMembers(ctx, fx.spaceId, []crypto.PubKey{fx.member})
		require.NoError(t, err)

		st := fx.childAcl.AclState()
		require.Equal(t, headId, st.LastRecordId())
		require.True(t, st.Permissions(fx.member).NoPermissions())
		require.True(t, st.RotationPending())
		require.True(t, st.Permissions(fx.parentKeys.SignKey.GetPublic()).NoPermissions())
	})

	t.Run("the coordinator's refusal is returned", func(t *testing.T) {
		fx := newParentFixture(t)
		fx.expectAdd(t, list.ParentAuthority{})

		_, err := fx.RemoveChildMembers(ctx, fx.spaceId, []crypto.PubKey{fx.member})
		require.ErrorIs(t, err, list.ErrInsufficientPermissions)
		require.True(t, fx.childAcl.AclState().Permissions(fx.member).CanWrite())
	})

	t.Run("the owner of the child is refused before sending", func(t *testing.T) {
		fx := newParentFixture(t)
		_, err := fx.RemoveChildMembers(ctx, fx.spaceId, []crypto.PubKey{fx.owner})
		require.ErrorIs(t, err, list.ErrInsufficientPermissions)
	})
}

func TestAclJoiningClient_ChangeChildOwner(t *testing.T) {
	t.Run("the owner of the parent hands the child to a member", func(t *testing.T) {
		fx := newParentFixture(t)
		fx.expectAdd(t, list.ParentAuthority{ChangeOwner: true})

		headId, err := fx.ChangeChildOwner(ctx, fx.spaceId, fx.member, list.AclPermissionsAdmin)
		require.NoError(t, err)

		st := fx.childAcl.AclState()
		require.Equal(t, headId, st.LastRecordId())
		require.True(t, st.Permissions(fx.member).IsOwner())
		require.Equal(t, list.AclPermissionsAdmin, st.Permissions(fx.owner))
		require.True(t, st.Permissions(fx.parentKeys.SignKey.GetPublic()).NoPermissions())
	})

	t.Run("the coordinator's refusal is returned", func(t *testing.T) {
		fx := newParentFixture(t)
		fx.expectAdd(t, list.ParentAuthority{RemoveMembers: true})

		_, err := fx.ChangeChildOwner(ctx, fx.spaceId, fx.member, list.AclPermissionsAdmin)
		require.ErrorIs(t, err, list.ErrInsufficientPermissions)
		require.True(t, fx.childAcl.AclState().Permissions(fx.owner).IsOwner())
	})

	t.Run("a new owner who is not a member of the child is refused before sending", func(t *testing.T) {
		fx := newParentFixture(t)
		_, err := fx.ChangeChildOwner(ctx, fx.spaceId, fx.parentKeys.SignKey.GetPublic(), list.AclPermissionsAdmin)
		require.ErrorIs(t, err, list.ErrNoSuchAccount)
	})
}
