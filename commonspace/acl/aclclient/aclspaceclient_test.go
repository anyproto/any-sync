package aclclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
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
	data, err := rec.Marshal()
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
