package commonspace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/config"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/spacepayloads"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/nodeconf/testconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/syncqueues"
)

func TestSpaceService_SpacePull(t *testing.T) {
	var makeClientServer = func(t *testing.T) (fxC, fxS *spacePullFixture, peerId string) {
		fxC = newSpacePullFixture(t)
		fxS = newSpacePullFixture(t)
		peerId = "peer"
		// Use the server account's peerId as the client connection peerId
		// so the server's nodeconf recognizes the client as a known network node
		clientPeerId := fxS.account.Account().PeerId
		mcS, mcC := rpctest.MultiConnPair(peerId, clientPeerId)
		pS, err := peer.NewPeer(mcS, fxC.ts)
		require.NoError(t, err)
		fxC.tp.AddPeer(ctx, pS)
		_, err = peer.NewPeer(mcC, fxS.ts)
		fxC.managerProvider.peer = pS
		require.NoError(t, err)
		return
	}

	t.Run("successful space pull", func(t *testing.T) {
		fxC, fxS, _ := makeClientServer(t)
		defer fxC.Finish(t)
		defer fxS.Finish(t)

		spaceId, payload := fxS.createTestSpace(t)

		space, err := fxC.spaceService.NewSpace(ctx, spaceId, Deps{
			SyncStatus:     syncstatus.NewNoOpSyncStatus(),
			TreeSyncer:     &mockTreeSyncer{},
			AccountService: fxC.account,
		})
		require.NoError(t, err)
		require.NoError(t, space.Init(ctx))
		require.NotNil(t, space)
		require.Equal(t, spaceId, space.Id())

		storage := space.Storage()
		state, err := storage.StateStorage().GetState(ctx)
		require.NoError(t, err)
		require.Equal(t, spaceId, state.SpaceId)
		require.Equal(t, payload.SpaceHeaderWithId.Id, state.SpaceId)
	})

	t.Run("space pull with acl records", func(t *testing.T) {
		fxC, fxS, _ := makeClientServer(t)
		defer fxC.Finish(t)
		defer fxS.Finish(t)

		spaceId, recLen, payload := fxS.createTestSpaceWithAclRecords(t)

		space, err := fxC.spaceService.NewSpace(ctx, spaceId, Deps{
			SyncStatus:     syncstatus.NewNoOpSyncStatus(),
			TreeSyncer:     &mockTreeSyncer{},
			AccountService: fxC.account,
			recordVerifier: recordverifier.NewValidateFull(),
		})
		require.NoError(t, err)
		require.NotNil(t, space)
		require.NoError(t, space.Init(ctx))
		records, err := space.Acl().RecordsAfter(ctx, "")
		require.NoError(t, err)
		require.Equal(t, recLen, len(records))
		storage := space.Storage()
		state, err := storage.StateStorage().GetState(ctx)
		require.NoError(t, err)
		require.Equal(t, spaceId, state.SpaceId)
		require.Equal(t, payload.SpaceHeaderWithId.Id, state.SpaceId)
	})
}

type spacePullFixture struct {
	*spaceService
	app             *app.App
	ctrl            *gomock.Controller
	ts              *rpctest.TestServer
	tp              *rpctest.TestPool
	tmpDir          string
	account         *accounttest.AccountTestService
	configService   *testconf.StubConf
	storage         spacestorage.SpaceStorageProvider
	managerProvider *mockPeerManagerProvider
}

func newSpacePullFixture(t *testing.T) (fx *spacePullFixture) {
	ts := rpctest.NewTestServer()
	fx = &spacePullFixture{
		spaceService:    New().(*spaceService),
		ctrl:            gomock.NewController(t),
		app:             new(app.App),
		ts:              ts,
		tp:              rpctest.NewTestPool(),
		tmpDir:          t.TempDir(),
		account:         &accounttest.AccountTestService{},
		configService:   &testconf.StubConf{},
		storage:         &spaceStorageProvider{rootPath: t.TempDir()},
		managerProvider: &mockPeerManagerProvider{},
	}

	configGetter := &mockSpaceConfigGetter{}

	fx.app.Register(configGetter).
		Register(mockCoordinatorClient{}).
		Register(mockNodeClient{}).
		Register(credentialprovider.NewNoOp()).
		Register(streampool.New()).
		Register(newStreamOpener("spaceId")).
		Register(fx.account).
		Register(fx.storage).
		Register(fx.managerProvider).
		Register(syncqueues.New()).
		Register(&mockTreeManager{}).
		Register(fx.spaceService).
		Register(fx.tp).
		Register(fx.ts).
		Register(fx.configService)

	require.NoError(t, spacesyncproto.DRPCRegisterSpaceSync(ts, &testSpaceSyncServer{spaceService: fx.spaceService}))
	require.NoError(t, fx.app.Start(ctx))

	return fx
}

func (fx *spacePullFixture) Finish(t *testing.T) {
	fx.ctrl.Finish()
}

func (fx *spacePullFixture) createTestSpace(t *testing.T) (string, spacestorage.SpaceStorageCreatePayload) {
	keys := fx.account.Account()
	masterKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	metaKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	require.NoError(t, err)
	readKey := crypto.NewAES()
	metadata := []byte("metadata")

	payload := spacepayloads.SpaceCreatePayload{
		SigningKey:     keys.SignKey,
		SpaceType:      "test",
		ReplicationKey: 1,
		SpacePayload:   nil,
		MasterKey:      masterKey,
		ReadKey:        readKey,
		MetadataKey:    metaKey,
		Metadata:       metadata,
	}

	createPayload, err := spacepayloads.StoragePayloadForSpaceCreate(payload)
	require.NoError(t, err)
	storage, err := fx.storage.CreateSpaceStorage(ctx, createPayload)
	require.NoError(t, err)
	require.NoError(t, storage.Close(ctx))

	return createPayload.SpaceHeaderWithId.Id, createPayload
}

func (fx *spacePullFixture) createTestSpaceWithAclRecords(t *testing.T) (string, int, spacestorage.SpaceStorageCreatePayload) {
	spaceId, payload := fx.createTestSpace(t)
	keys := fx.account.Account()

	executor := list.NewExternalKeysAclExecutor(spaceId, keys, []byte("metadata"), payload.AclWithId)
	cmds := []string{
		"0.init::0",
		"0.invite::invId1",
		"0.invite::invId2",
	}

	for _, cmd := range cmds {
		err := executor.Execute(cmd)
		require.NoError(t, err)
	}

	allRecords, err := executor.ActualAccounts()["0"].Acl.RecordsAfter(ctx, "")
	require.NoError(t, err)

	storage, err := fx.storage.WaitSpaceStorage(ctx, spaceId)
	require.NoError(t, err)
	defer storage.Close(ctx)

	aclStorage, err := storage.AclStorage()
	require.NoError(t, err)

	for i, rec := range allRecords[1:] { // Skip first record as it's already there
		err := aclStorage.AddAll(ctx, []list.StorageRecord{
			{
				RawRecord:  rec.Payload,
				Id:         rec.Id,
				PrevId:     allRecords[i].Id,
				Order:      i + 2,
				ChangeSize: len(rec.Payload),
			},
		})
		require.NoError(t, err)
	}

	return spaceId, len(allRecords), payload
}

type testSpaceSyncServer struct {
	spacesyncproto.DRPCSpaceSyncUnimplementedServer
	spaceService SpaceService
}

func (t *testSpaceSyncServer) SpacePull(ctx context.Context, req *spacesyncproto.SpacePullRequest) (*spacesyncproto.SpacePullResponse, error) {
	sp, err := t.spaceService.NewSpace(ctx, req.Id, Deps{
		SyncStatus:     syncstatus.NewNoOpSyncStatus(),
		TreeSyncer:     mockTreeSyncer{},
		recordVerifier: recordverifier.NewValidateFull(),
	})
	if err != nil {
		return nil, err
	}
	err = sp.Init(ctx)
	if err != nil {
		return nil, err
	}
	spaceDesc, err := sp.Description(ctx)
	if err != nil {
		return nil, err
	}

	return &spacesyncproto.SpacePullResponse{
		Payload: &spacesyncproto.SpacePayload{
			SpaceHeader:            spaceDesc.SpaceHeader,
			AclPayloadId:           spaceDesc.AclId,
			AclPayload:             spaceDesc.AclPayload,
			SpaceSettingsPayload:   spaceDesc.SpaceSettingsPayload,
			SpaceSettingsPayloadId: spaceDesc.SpaceSettingsId,
		},
		AclRecords: spaceDesc.AclRecords,
	}, nil
}

type mockSpaceConfigGetter struct{}

func (m *mockSpaceConfigGetter) GetStreamConfig() streampool.StreamConfig {
	return streampool.StreamConfig{}
}

func (m *mockSpaceConfigGetter) Init(a *app.App) error { return nil }
func (m *mockSpaceConfigGetter) Name() string          { return "config" }
func (m *mockSpaceConfigGetter) GetSpace() config.Config {
	return config.Config{
		GCTTL:                60,
		SyncPeriod:           5,
		KeepTreeDataInMemory: true,
	}
}

type mockTreeManager struct{}

func (m *mockTreeManager) Init(a *app.App) error           { return nil }
func (m *mockTreeManager) Name() string                    { return treemanager.CName }
func (m *mockTreeManager) Run(ctx context.Context) error   { return nil }
func (m *mockTreeManager) Close(ctx context.Context) error { return nil }
func (m *mockTreeManager) GetTree(ctx context.Context, spaceId, treeId string) (objecttree.ObjectTree, error) {
	return nil, nil
}
func (m *mockTreeManager) CreateTree(ctx context.Context, spaceId string) (objecttree.ObjectTree, error) {
	return nil, nil
}
func (m *mockTreeManager) PutTree(ctx context.Context, payload treestorage.TreeStorageCreatePayload) (objecttree.ObjectTree, error) {
	return nil, nil
}
func (m *mockTreeManager) DeleteTree(ctx context.Context, spaceId, treeId string) error { return nil }
func (m *mockTreeManager) MarkTreeDeleted(ctx context.Context, spaceId, treeId string) error {
	return nil
}
func (m *mockTreeManager) ValidateAndPutTree(ctx context.Context, spaceId string, payload treestorage.TreeStorageCreatePayload) error {
	return nil
}
