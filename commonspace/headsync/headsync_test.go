package headsync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/ldiff/mock_ldiff"
	"github.com/anyproto/any-sync/commonspace/config"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/credentialprovider/mock_credentialprovider"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/deletionstate/mock_deletionstate"
	"github.com/anyproto/any-sync/commonspace/headsync/mock_headsync"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/mock_syncacl"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage/mock_treestorage"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/object/treemanager/mock_treemanager"
	"github.com/anyproto/any-sync/commonspace/object/treesyncer"
	"github.com/anyproto/any-sync/commonspace/object/treesyncer/mock_treesyncer"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/peermanager/mock_peermanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage/mock_spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto/mock_spacesyncproto"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/mock_nodeconf"
)

type mockConfig struct {
}

func (m mockConfig) Init(a *app.App) (err error) {
	return nil
}

func (m mockConfig) Name() (name string) {
	return "config"
}

func (m mockConfig) GetSpace() config.Config {
	return config.Config{}
}

type headSyncFixture struct {
	spaceState *spacestate.SpaceState
	ctrl       *gomock.Controller
	app        *app.App

	configurationMock      *mock_nodeconf.MockService
	storageMock            *mock_spacestorage.MockSpaceStorage
	peerManagerMock        *mock_peermanager.MockPeerManager
	credentialProviderMock *mock_credentialprovider.MockCredentialProvider
	treeManagerMock        *mock_treemanager.MockTreeManager
	deletionStateMock      *mock_deletionstate.MockObjectDeletionState
	diffSyncerMock         *mock_headsync.MockDiffSyncer
	treeSyncerMock         *mock_treesyncer.MockTreeSyncer
	diffMock               *mock_ldiff.MockDiff
	clientMock             *mock_spacesyncproto.MockDRPCSpaceSyncClient
	aclMock                *mock_syncacl.MockSyncAcl
	headSync               *headSync
	diffSyncer             *diffSyncer
}

func newHeadSyncFixture(t *testing.T) *headSyncFixture {
	spaceState := &spacestate.SpaceState{
		SpaceId: "spaceId",
	}
	ctrl := gomock.NewController(t)
	configurationMock := mock_nodeconf.NewMockService(ctrl)
	configurationMock.EXPECT().Name().AnyTimes().Return(nodeconf.CName)
	storageMock := mock_spacestorage.NewMockSpaceStorage(ctrl)
	storageMock.EXPECT().Name().AnyTimes().Return(spacestorage.CName)
	peerManagerMock := mock_peermanager.NewMockPeerManager(ctrl)
	peerManagerMock.EXPECT().Name().AnyTimes().Return(peermanager.CName)
	credentialProviderMock := mock_credentialprovider.NewMockCredentialProvider(ctrl)
	credentialProviderMock.EXPECT().Name().AnyTimes().Return(credentialprovider.CName)
	treeManagerMock := mock_treemanager.NewMockTreeManager(ctrl)
	treeManagerMock.EXPECT().Name().AnyTimes().Return(treemanager.CName)
	deletionStateMock := mock_deletionstate.NewMockObjectDeletionState(ctrl)
	deletionStateMock.EXPECT().Name().AnyTimes().Return(deletionstate.CName)
	diffSyncerMock := mock_headsync.NewMockDiffSyncer(ctrl)
	treeSyncerMock := mock_treesyncer.NewMockTreeSyncer(ctrl)
	treeSyncerMock.EXPECT().Name().AnyTimes().Return(treesyncer.CName)
	diffMock := mock_ldiff.NewMockDiff(ctrl)
	clientMock := mock_spacesyncproto.NewMockDRPCSpaceSyncClient(ctrl)
	aclMock := mock_syncacl.NewMockSyncAcl(ctrl)
	aclMock.EXPECT().Name().AnyTimes().Return(syncacl.CName)
	aclMock.EXPECT().SetHeadUpdater(gomock.Any()).AnyTimes()

	hs := &headSync{}
	a := &app.App{}
	a.Register(spaceState).
		Register(aclMock).
		Register(mockConfig{}).
		Register(configurationMock).
		Register(storageMock).
		Register(peerManagerMock).
		Register(credentialProviderMock).
		Register(treeManagerMock).
		Register(treeSyncerMock).
		Register(deletionStateMock).
		Register(hs)
	return &headSyncFixture{
		spaceState:             spaceState,
		ctrl:                   ctrl,
		app:                    a,
		configurationMock:      configurationMock,
		storageMock:            storageMock,
		peerManagerMock:        peerManagerMock,
		credentialProviderMock: credentialProviderMock,
		treeManagerMock:        treeManagerMock,
		deletionStateMock:      deletionStateMock,
		headSync:               hs,
		diffSyncerMock:         diffSyncerMock,
		treeSyncerMock:         treeSyncerMock,
		diffMock:               diffMock,
		clientMock:             clientMock,
		aclMock:                aclMock,
	}
}

func (fx *headSyncFixture) init(t *testing.T) {
	createDiffSyncer = func(hs *headSync) DiffSyncer {
		return fx.diffSyncerMock
	}
	fx.diffSyncerMock.EXPECT().Init()
	err := fx.headSync.Init(fx.app)
	require.NoError(t, err)
	fx.headSync.diff = fx.diffMock
}

func (fx *headSyncFixture) stop() {
	fx.ctrl.Finish()
}

func TestHeadSync(t *testing.T) {
	ctx := context.Background()

	t.Run("run close", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.init(t)
		defer fx.stop()

		ids := []string{"id1"}
		treeMock := mock_treestorage.NewMockTreeStorage(fx.ctrl)
		fx.storageMock.EXPECT().StoredIds().Return(ids, nil)
		fx.storageMock.EXPECT().TreeStorage(ids[0]).Return(treeMock, nil)
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.aclMock.EXPECT().Head().AnyTimes().Return(&list.AclRecord{Id: "headId"})
		treeMock.EXPECT().Heads().Return([]string{"h1", "h2"}, nil)
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: "h1h2",
		})
		fx.diffMock.EXPECT().Hash().Return("hash")
		fx.storageMock.EXPECT().WriteSpaceHash("hash").Return(nil)
		fx.diffSyncerMock.EXPECT().Sync(gomock.Any()).Return(nil)
		err := fx.headSync.Run(ctx)
		require.NoError(t, err)
		err = fx.headSync.Close(ctx)
		require.NoError(t, err)
	})

	t.Run("update heads", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.init(t)
		defer fx.stop()

		fx.diffSyncerMock.EXPECT().UpdateHeads("id1", []string{"h1"})
		fx.headSync.UpdateHeads("id1", []string{"h1"})
	})

	t.Run("remove objects", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.init(t)
		defer fx.stop()

		fx.diffSyncerMock.EXPECT().RemoveObjects([]string{"id1"})
		fx.headSync.RemoveObjects([]string{"id1"})
	})
}
