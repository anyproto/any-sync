package commonspace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/testconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/syncqueues"
)

func newACLTestFixture(t *testing.T, additionalNodes ...nodeconf.Node) *spacePullFixture {
	ts := rpctest.NewTestServer()
	configService := &testconf.StubConf{AdditionalNodes: additionalNodes}
	account := &accounttest.AccountTestService{}
	storage := &spaceStorageProvider{rootPath: t.TempDir()}

	fx := &spacePullFixture{
		spaceService:    New().(*spaceService),
		ctrl:            gomock.NewController(t),
		app:             new(app.App),
		ts:              ts,
		tp:              rpctest.NewTestPool(),
		tmpDir:          t.TempDir(),
		account:         account,
		configService:   configService,
		storage:         storage,
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

func TestCheckReadAccess(t *testing.T) {
	t.Run("owner can read", func(t *testing.T) {
		fx := newACLTestFixture(t)
		defer fx.Finish(t)

		spaceId, _ := fx.createTestSpace(t)
		sp, err := fx.spaceService.NewSpace(ctx, spaceId, Deps{
			SyncStatus:     syncstatus.NewNoOpSyncStatus(),
			TreeSyncer:     mockTreeSyncer{},
			recordVerifier: recordverifier.NewValidateFull(),
		})
		require.NoError(t, err)
		require.NoError(t, sp.Init(ctx))

		ownerKeys := fx.account.Account()
		identity, err := ownerKeys.SignKey.GetPublic().Marshall()
		require.NoError(t, err)

		peerCtx := peer.CtxWithIdentity(peer.CtxWithPeerId(context.Background(), ownerKeys.PeerId), identity)

		_, err = sp.Description(peerCtx)
		require.NoError(t, err)
	})

	t.Run("acl member with read permission can read", func(t *testing.T) {
		fx := newACLTestFixture(t)
		defer fx.Finish(t)

		spaceId, payload := fx.createTestSpace(t)

		// Add a reader to the ACL
		ownerKeys := fx.account.Account()
		executor := list.NewExternalKeysAclExecutor(spaceId, ownerKeys, []byte("metadata"), payload.AclWithId)
		cmds := []string{
			"0.init::0",
			"0.invite::invId",
			"1.join::invId",
			"0.approve::1,r",
		}
		for _, cmd := range cmds {
			err := executor.Execute(cmd)
			require.NoError(t, err, cmd)
		}

		// Store additional ACL records
		allRecords, err := executor.ActualAccounts()["0"].Acl.RecordsAfter(context.Background(), "")
		require.NoError(t, err)
		storage, err := fx.storage.WaitSpaceStorage(ctx, spaceId)
		require.NoError(t, err)
		aclStorage, err := storage.AclStorage()
		require.NoError(t, err)
		for i, rec := range allRecords[1:] {
			err := aclStorage.AddAll(ctx, []list.StorageRecord{
				{RawRecord: rec.Payload, Id: rec.Id, PrevId: allRecords[i].Id, Order: i + 2, ChangeSize: len(rec.Payload)},
			})
			require.NoError(t, err)
		}
		storage.Close(ctx)

		sp, err := fx.spaceService.NewSpace(ctx, spaceId, Deps{
			SyncStatus:     syncstatus.NewNoOpSyncStatus(),
			TreeSyncer:     mockTreeSyncer{},
			recordVerifier: recordverifier.NewValidateFull(),
		})
		require.NoError(t, err)
		require.NoError(t, sp.Init(ctx))

		readerKeys := executor.ActualAccounts()["1"].Keys
		readerIdentity, err := readerKeys.SignKey.GetPublic().Marshall()
		require.NoError(t, err)

		peerCtx := peer.CtxWithIdentity(peer.CtxWithPeerId(context.Background(), readerKeys.PeerId), readerIdentity)

		_, err = sp.Description(peerCtx)
		require.NoError(t, err)
	})

	t.Run("network node can read without acl entry", func(t *testing.T) {
		nodePeerId := "known-node-peer"
		fx := newACLTestFixture(t, nodeconf.Node{
			PeerId:    nodePeerId,
			Addresses: []string{"127.0.0.1:4431"},
			Types:     []nodeconf.NodeType{nodeconf.NodeTypeTree},
		})
		defer fx.Finish(t)

		spaceId, _ := fx.createTestSpace(t)
		sp, err := fx.spaceService.NewSpace(ctx, spaceId, Deps{
			SyncStatus:     syncstatus.NewNoOpSyncStatus(),
			TreeSyncer:     mockTreeSyncer{},
			recordVerifier: recordverifier.NewValidateFull(),
		})
		require.NoError(t, err)
		require.NoError(t, sp.Init(ctx))

		peerCtx := peer.CtxWithPeerId(context.Background(), nodePeerId)

		_, err = sp.Description(peerCtx)
		require.NoError(t, err)
	})

	t.Run("unknown peer with identity but no acl entry gets forbidden", func(t *testing.T) {
		fx := newACLTestFixture(t)
		defer fx.Finish(t)

		spaceId, _ := fx.createTestSpace(t)
		sp, err := fx.spaceService.NewSpace(ctx, spaceId, Deps{
			SyncStatus:     syncstatus.NewNoOpSyncStatus(),
			TreeSyncer:     mockTreeSyncer{},
			recordVerifier: recordverifier.NewValidateFull(),
		})
		require.NoError(t, err)
		require.NoError(t, sp.Init(ctx))

		unknownKey, _, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		unknownIdentity, err := unknownKey.GetPublic().Marshall()
		require.NoError(t, err)

		peerCtx := peer.CtxWithIdentity(peer.CtxWithPeerId(context.Background(), "unknown-peer"), unknownIdentity)

		_, err = sp.Description(peerCtx)
		require.ErrorIs(t, err, spacesyncproto.ErrForbidden)
	})

	t.Run("peer with no identity gets forbidden", func(t *testing.T) {
		fx := newACLTestFixture(t)
		defer fx.Finish(t)

		spaceId, _ := fx.createTestSpace(t)
		sp, err := fx.spaceService.NewSpace(ctx, spaceId, Deps{
			SyncStatus:     syncstatus.NewNoOpSyncStatus(),
			TreeSyncer:     mockTreeSyncer{},
			recordVerifier: recordverifier.NewValidateFull(),
		})
		require.NoError(t, err)
		require.NoError(t, sp.Init(ctx))

		peerCtx := peer.CtxWithPeerId(context.Background(), "some-peer")

		_, err = sp.Description(peerCtx)
		require.ErrorIs(t, err, spacesyncproto.ErrForbidden)
	})

	t.Run("no peer id gets forbidden", func(t *testing.T) {
		fx := newACLTestFixture(t)
		defer fx.Finish(t)

		spaceId, _ := fx.createTestSpace(t)
		sp, err := fx.spaceService.NewSpace(ctx, spaceId, Deps{
			SyncStatus:     syncstatus.NewNoOpSyncStatus(),
			TreeSyncer:     mockTreeSyncer{},
			recordVerifier: recordverifier.NewValidateFull(),
		})
		require.NoError(t, err)
		require.NoError(t, sp.Init(ctx))

		_, err = sp.Description(context.Background())
		require.ErrorIs(t, err, spacesyncproto.ErrForbidden)
	})
}
