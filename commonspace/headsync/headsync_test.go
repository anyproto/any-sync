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
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage/mock_headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/mock_headsync"
	"github.com/anyproto/any-sync/commonspace/headsync/statestorage/mock_statestorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/mock_syncacl"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/mock_keyvaluestorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/kvinterfaces"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/kvinterfaces/mock_kvinterfaces"
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
	"github.com/anyproto/any-sync/testutil/anymock"
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
	kvMock                 *mock_kvinterfaces.MockKeyValueService
	defStoreMock           *mock_keyvaluestorage.MockStorage
	storageMock            *mock_spacestorage.MockSpaceStorage
	peerManagerMock        *mock_peermanager.MockPeerManager
	credentialProviderMock *mock_credentialprovider.MockCredentialProvider
	treeManagerMock        *mock_treemanager.MockTreeManager
	deletionStateMock      *mock_deletionstate.MockObjectDeletionState
	diffSyncerMock         *mock_headsync.MockDiffSyncer
	treeSyncerMock         *mock_treesyncer.MockTreeSyncer
	diffMock               *mock_ldiff.MockDiff
	diffContainerMock      *mock_ldiff.MockDiffContainer
	clientMock             *mock_spacesyncproto.MockDRPCSpaceSyncClient
	aclMock                *mock_syncacl.MockSyncAcl
	headStorage            *mock_headstorage.MockHeadStorage
	stateStorage           *mock_statestorage.MockStateStorage
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
	diffContainerMock := mock_ldiff.NewMockDiffContainer(ctrl)
	treeSyncerMock := mock_treesyncer.NewMockTreeSyncer(ctrl)
	headStorage := mock_headstorage.NewMockHeadStorage(ctrl)
	stateStorage := mock_statestorage.NewMockStateStorage(ctrl)
	kvMock := mock_kvinterfaces.NewMockKeyValueService(ctrl)
	anymock.ExpectComp(kvMock.EXPECT(), kvinterfaces.CName)
	defStore := mock_keyvaluestorage.NewMockStorage(ctrl)
	kvMock.EXPECT().DefaultStore().Return(defStore).AnyTimes()
	defStore.EXPECT().Id().Return("store").AnyTimes()
	storageMock.EXPECT().HeadStorage().AnyTimes().Return(headStorage)
	storageMock.EXPECT().StateStorage().AnyTimes().Return(stateStorage)
	treeSyncerMock.EXPECT().Name().AnyTimes().Return(treesyncer.CName)
	diffMock := mock_ldiff.NewMockDiff(ctrl)
	clientMock := mock_spacesyncproto.NewMockDRPCSpaceSyncClient(ctrl)
	aclMock := mock_syncacl.NewMockSyncAcl(ctrl)
	aclMock.EXPECT().Name().AnyTimes().Return(syncacl.CName)

	hs := &headSync{}
	a := &app.App{}
	a.Register(spaceState).
		Register(aclMock).
		Register(kvMock).
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
		kvMock:                 kvMock,
		defStoreMock:           defStore,
		configurationMock:      configurationMock,
		storageMock:            storageMock,
		diffContainerMock:      diffContainerMock,
		peerManagerMock:        peerManagerMock,
		credentialProviderMock: credentialProviderMock,
		treeManagerMock:        treeManagerMock,
		deletionStateMock:      deletionStateMock,
		headStorage:            headStorage,
		stateStorage:           stateStorage,
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
	fx.headStorage.EXPECT().AddObserver(gomock.Any())
	err := fx.headSync.Init(fx.app)
	require.NoError(t, err)
	fx.headSync.diffContainer = fx.diffContainerMock
}

func (fx *headSyncFixture) stop() {
	fx.ctrl.Finish()
}

func TestHeadSync(t *testing.T) {
	ctx := context.Background()

	t.Run("run close", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()

		headEntries := []headstorage.HeadsEntry{
			{
				Id:             "id1",
				Heads:          []string{"h1", "h2"},
				CommonSnapshot: "id1",
				IsDerived:      false,
			},
			{
				Id:             "id2",
				Heads:          []string{"h3", "h4"},
				CommonSnapshot: "id2",
				IsDerived:      false,
			},
		}
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, opts headstorage.IterOpts, entryIter headstorage.EntryIterator) error {
				for _, entry := range headEntries {
					if res, err := entryIter(entry); err != nil || !res {
						return err
					}
				}
				return nil
			})
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.aclMock.EXPECT().Head().AnyTimes().Return(&list.AclRecord{Id: "headId"})

		fx.diffContainerMock.EXPECT().Set(ldiff.Element{
			Id:   "id1",
			Head: "h1h2",
		}, ldiff.Element{
			Id:   "id2",
			Head: "h3h4",
		}, ldiff.Element{
			Id:   "aclId",
			Head: "headId",
		})
		fx.diffMock.EXPECT().Set([]ldiff.Element{})
		fx.diffContainerMock.EXPECT().NewDiff().AnyTimes().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().OldDiff().AnyTimes().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().AnyTimes().Return("hash")
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash", "hash").Return(nil)
		fx.diffSyncerMock.EXPECT().Sync(gomock.Any()).Return(nil)
		fx.diffSyncerMock.EXPECT().Close()
		err := fx.headSync.Run(ctx)
		require.NoError(t, err)
		err = fx.headSync.Close(ctx)
		require.NoError(t, err)
	})

	t.Run("run close, no empty derived", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()

		headEntries := []headstorage.HeadsEntry{
			{
				Id:             "id1",
				Heads:          []string{"id1"},
				CommonSnapshot: "id1",
				IsDerived:      true,
			},
			{
				Id:             "id2",
				Heads:          []string{"h3", "h4"},
				CommonSnapshot: "id2",
				IsDerived:      false,
			},
		}
		fx.headStorage.EXPECT().IterateEntries(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, opts headstorage.IterOpts, entryIter headstorage.EntryIterator) error {
				for _, entry := range headEntries {
					if res, err := entryIter(entry); err != nil || !res {
						return err
					}
				}
				return nil
			})
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.aclMock.EXPECT().Head().AnyTimes().Return(&list.AclRecord{Id: "headId"})

		fx.diffContainerMock.EXPECT().Set(ldiff.Element{
			Id:   "id2",
			Head: "h3h4",
		}, ldiff.Element{
			Id:   "aclId",
			Head: "headId",
		})
		fx.diffMock.EXPECT().Set([]ldiff.Element{})
		fx.diffContainerMock.EXPECT().NewDiff().AnyTimes().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().OldDiff().AnyTimes().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().AnyTimes().Return("hash")
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash", "hash").Return(nil)
		fx.diffSyncerMock.EXPECT().Sync(gomock.Any()).Return(nil)
		fx.diffSyncerMock.EXPECT().Close()
		err := fx.headSync.Run(ctx)
		require.NoError(t, err)
		err = fx.headSync.Close(ctx)
		require.NoError(t, err)
	})
}
