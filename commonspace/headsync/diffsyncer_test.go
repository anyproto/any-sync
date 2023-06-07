package headsync

import (
	"context"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"storj.io/drpc"
	"testing"
	"time"
)

type mockPeer struct {
}

func (m mockPeer) Id() string {
	return "peerId"
}

func (m mockPeer) Context() context.Context {
	return context.Background()
}

func (m mockPeer) AcquireDrpcConn(ctx context.Context) (drpc.Conn, error) {
	return nil, nil
}

func (m mockPeer) ReleaseDrpcConn(conn drpc.Conn) {
	return
}

func (m mockPeer) DoDrpc(ctx context.Context, do func(conn drpc.Conn) error) error {
	return nil
}

func (m mockPeer) IsClosed() bool {
	return false
}

func (m mockPeer) TryClose(objectTTL time.Duration) (res bool, err error) {
	return false, err
}

func (m mockPeer) Close() (err error) {
	return nil
}

func (fx *headSyncFixture) initDiffSyncer(t *testing.T) {
	fx.init(t)
	fx.diffSyncer = newDiffSyncer(fx.headSync).(*diffSyncer)
	fx.diffSyncer.clientFactory = spacesyncproto.ClientFactoryFunc(func(cc drpc.Conn) spacesyncproto.DRPCSpaceSyncClient {
		return fx.clientMock
	})
	fx.deletionStateMock.EXPECT().AddObserver(gomock.Any())
	fx.treeManagerMock.EXPECT().NewTreeSyncer(fx.spaceState.SpaceId, fx.treeManagerMock).Return(fx.treeSyncerMock)
	fx.diffSyncer.Init()
}

func TestDiffSyncer(t *testing.T) {
	fx := newHeadSyncFixture(t)
	fx.initDiffSyncer(t)
	defer fx.stop()
	ctx := context.Background()

	t.Run("diff syncer sync", func(t *testing.T) {
		mPeer := mockPeer{}
		fx.peerManagerMock.EXPECT().
			GetResponsiblePeers(gomock.Any()).
			Return([]peer.Peer{mPeer}, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(NewRemoteDiff(fx.spaceState.SpaceId, fx.clientMock))).
			Return([]string{"new"}, []string{"changed"}, nil, nil)
		fx.deletionStateMock.EXPECT().Filter([]string{"new"}).Return([]string{"new"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter([]string{"changed"}).Return([]string{"changed"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter(nil).Return(nil).Times(1)
		fx.treeSyncerMock.EXPECT().SyncAll(gomock.Any(), mPeer.Id(), []string{"changed"}, []string{"new"}).Return(nil)
		require.NoError(t, fx.diffSyncer.Sync(ctx))
	})
}

//
//type pushSpaceRequestMatcher struct {
//	spaceId     string
//	aclRootId   string
//	settingsId  string
//	credential  []byte
//	spaceHeader *spacesyncproto.RawSpaceHeaderWithId
//}
//
//func (p pushSpaceRequestMatcher) Matches(x interface{}) bool {
//	res, ok := x.(*spacesyncproto.SpacePushRequest)
//	if !ok {
//		return false
//	}
//
//	return res.Payload.AclPayloadId == p.aclRootId && res.Payload.SpaceHeader == p.spaceHeader && res.Payload.SpaceSettingsPayloadId == p.settingsId && bytes.Equal(p.credential, res.Credential)
//}
//
//func (p pushSpaceRequestMatcher) String() string {
//	return ""
//}
//
//type mockPeer struct{}
//
//func (m mockPeer) Addr() string {
//	return ""
//}
//
//func (m mockPeer) TryClose(objectTTL time.Duration) (res bool, err error) {
//	return true, m.Close()
//}
//
//func (m mockPeer) Id() string {
//	return "mockId"
//}
//
//func (m mockPeer) LastUsage() time.Time {
//	return time.Time{}
//}
//
//func (m mockPeer) Secure() sec.SecureConn {
//	return nil
//}
//
//func (m mockPeer) UpdateLastUsage() {
//}
//
//func (m mockPeer) Close() error {
//	return nil
//}
//
//func (m mockPeer) Closed() <-chan struct{} {
//	return make(chan struct{})
//}
//
//func (m mockPeer) Invoke(ctx context.Context, rpc string, enc drpc.Encoding, in, out drpc.Message) error {
//	return nil
//}
//
//func (m mockPeer) NewStream(ctx context.Context, rpc string, enc drpc.Encoding) (drpc.Stream, error) {
//	return nil, nil
//}
//
//func newPushSpaceRequestMatcher(
//	spaceId string,
//	aclRootId string,
//	settingsId string,
//	credential []byte,
//	spaceHeader *spacesyncproto.RawSpaceHeaderWithId) *pushSpaceRequestMatcher {
//	return &pushSpaceRequestMatcher{
//		spaceId:     spaceId,
//		aclRootId:   aclRootId,
//		settingsId:  settingsId,
//		credential:  credential,
//		spaceHeader: spaceHeader,
//	}
//}
//
//func TestDiffSyncer_Sync(t *testing.T) {
//	// setup
//	fx := newHeadSyncFixture(t)
//	fx.initDiffSyncer(t)
//	defer fx.stop()
//
//	diffMock := mock_ldiff.NewMockDiff(ctrl)
//	peerManagerMock := mock_peermanager.NewMockPeerManager(ctrl)
//	cacheMock := mock_treemanager.NewMockTreeManager(ctrl)
//	stMock := mock_spacestorage.NewMockSpaceStorage(ctrl)
//	clientMock := mock_spacesyncproto.NewMockDRPCSpaceSyncClient(ctrl)
//	factory := spacesyncproto.ClientFactoryFunc(func(cc drpc.Conn) spacesyncproto.DRPCSpaceSyncClient {
//		return clientMock
//	})
//	treeSyncerMock := mock_treemanager.NewMockTreeSyncer(ctrl)
//	credentialProvider := mock_credentialprovider.NewMockCredentialProvider(ctrl)
//	delState := mock_settingsstate.NewMockObjectDeletionState(ctrl)
//	spaceId := "spaceId"
//	aclRootId := "aclRootId"
//	l := logger.NewNamed(spaceId)
//	diffSyncer := newDiffSyncer(spaceId, diffMock, peerManagerMock, cacheMock, stMock, factory, syncstatus.NewNoOpSyncStatus(), credentialProvider, l)
//	delState.EXPECT().AddObserver(gomock.Any())
//	cacheMock.EXPECT().NewTreeSyncer(spaceId, gomock.Any()).Return(treeSyncerMock)
//	diffSyncer.Init(delState)
//
//	t.Run("diff syncer sync", func(t *testing.T) {
//		mPeer := mockPeer{}
//		peerManagerMock.EXPECT().
//			GetResponsiblePeers(gomock.Any()).
//			Return([]peer.Peer{mPeer}, nil)
//		diffMock.EXPECT().
//			Diff(gomock.Any(), gomock.Eq(NewRemoteDiff(spaceId, clientMock))).
//			Return([]string{"new"}, []string{"changed"}, nil, nil)
//		delState.EXPECT().Filter([]string{"new"}).Return([]string{"new"}).Times(1)
//		delState.EXPECT().Filter([]string{"changed"}).Return([]string{"changed"}).Times(1)
//		delState.EXPECT().Filter(nil).Return(nil).Times(1)
//		treeSyncerMock.EXPECT().SyncAll(gomock.Any(), mPeer.Id(), []string{"changed"}, []string{"new"}).Return(nil)
//		require.NoError(t, diffSyncer.Sync(ctx))
//	})
//}
//
//	t.Run("diff syncer sync conf error", func(t *testing.T) {
//		peerManagerMock.EXPECT().
//			GetResponsiblePeers(gomock.Any()).
//			Return(nil, fmt.Errorf("some error"))
//
//		require.Error(t, diffSyncer.Sync(ctx))
//	})
//
//	t.Run("deletion state remove objects", func(t *testing.T) {
//		deletedId := "id"
//		delState.EXPECT().Exists(deletedId).Return(true)
//
//		// this should not result in any mock being called
//		diffSyncer.UpdateHeads(deletedId, []string{"someHead"})
//	})
//
//	t.Run("update heads updates diff", func(t *testing.T) {
//		newId := "newId"
//		newHeads := []string{"h1", "h2"}
//		hash := "hash"
//		diffMock.EXPECT().Set(ldiff.Element{
//			Id:   newId,
//			Head: concatStrings(newHeads),
//		})
//		diffMock.EXPECT().Hash().Return(hash)
//		delState.EXPECT().Exists(newId).Return(false)
//		stMock.EXPECT().WriteSpaceHash(hash)
//		diffSyncer.UpdateHeads(newId, newHeads)
//	})
//
//	t.Run("diff syncer sync space missing", func(t *testing.T) {
//		aclStorageMock := mock_liststorage.NewMockListStorage(ctrl)
//		settingsStorage := mock_treestorage.NewMockTreeStorage(ctrl)
//		settingsId := "settingsId"
//		aclRoot := &aclrecordproto.RawAclRecordWithId{
//			Id: aclRootId,
//		}
//		settingsRoot := &treechangeproto.RawTreeChangeWithId{
//			Id: settingsId,
//		}
//		spaceHeader := &spacesyncproto.RawSpaceHeaderWithId{}
//		spaceSettingsId := "spaceSettingsId"
//		credential := []byte("credential")
//
//		peerManagerMock.EXPECT().
//			GetResponsiblePeers(gomock.Any()).
//			Return([]peer.Peer{mockPeer{}}, nil)
//		diffMock.EXPECT().
//			Diff(gomock.Any(), gomock.Eq(NewRemoteDiff(spaceId, clientMock))).
//			Return(nil, nil, nil, spacesyncproto.ErrSpaceMissing)
//
//		stMock.EXPECT().AclStorage().Return(aclStorageMock, nil)
//		stMock.EXPECT().SpaceHeader().Return(spaceHeader, nil)
//		stMock.EXPECT().SpaceSettingsId().Return(spaceSettingsId)
//		stMock.EXPECT().TreeStorage(spaceSettingsId).Return(settingsStorage, nil)
//
//		settingsStorage.EXPECT().Root().Return(settingsRoot, nil)
//		aclStorageMock.EXPECT().
//			Root().
//			Return(aclRoot, nil)
//		credentialProvider.EXPECT().
//			GetCredential(gomock.Any(), spaceHeader).
//			Return(credential, nil)
//		clientMock.EXPECT().
//			SpacePush(gomock.Any(), newPushSpaceRequestMatcher(spaceId, aclRootId, settingsId, credential, spaceHeader)).
//			Return(nil, nil)
//		peerManagerMock.EXPECT().SendPeer(gomock.Any(), "mockId", gomock.Any())
//
//		require.NoError(t, diffSyncer.Sync(ctx))
//	})
//
//	t.Run("diff syncer sync unexpected", func(t *testing.T) {
//		peerManagerMock.EXPECT().
//			GetResponsiblePeers(gomock.Any()).
//			Return([]peer.Peer{mockPeer{}}, nil)
//		diffMock.EXPECT().
//			Diff(gomock.Any(), gomock.Eq(NewRemoteDiff(spaceId, clientMock))).
//			Return(nil, nil, nil, spacesyncproto.ErrUnexpected)
//
//		require.NoError(t, diffSyncer.Sync(ctx))
//	})
//
//	t.Run("diff syncer sync space is deleted error", func(t *testing.T) {
//		mPeer := mockPeer{}
//		peerManagerMock.EXPECT().
//			GetResponsiblePeers(gomock.Any()).
//			Return([]peer.Peer{mPeer}, nil)
//		diffMock.EXPECT().
//			Diff(gomock.Any(), gomock.Eq(NewRemoteDiff(spaceId, clientMock))).
//			Return(nil, nil, nil, spacesyncproto.ErrSpaceIsDeleted)
//		stMock.EXPECT().SpaceSettingsId().Return("settingsId")
//		treeSyncerMock.EXPECT().SyncAll(gomock.Any(), mPeer.Id(), []string{"settingsId"}, nil).Return(nil)
//
//		require.NoError(t, diffSyncer.Sync(ctx))
//	})
//}
