package headsync

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
)

type pushSpaceRequestMatcher struct {
	spaceId     string
	aclRootId   string
	settingsId  string
	credential  []byte
	spaceHeader *spacesyncproto.RawSpaceHeaderWithId
}

func newPushSpaceRequestMatcher(
	spaceId string,
	aclRootId string,
	settingsId string,
	credential []byte,
	spaceHeader *spacesyncproto.RawSpaceHeaderWithId) *pushSpaceRequestMatcher {
	return &pushSpaceRequestMatcher{
		spaceId:     spaceId,
		aclRootId:   aclRootId,
		settingsId:  settingsId,
		credential:  credential,
		spaceHeader: spaceHeader,
	}
}

func (p pushSpaceRequestMatcher) Matches(x interface{}) bool {
	res, ok := x.(*spacesyncproto.SpacePushRequest)
	if !ok {
		return false
	}

	return res.Payload.AclPayloadId == p.aclRootId && res.Payload.SpaceHeader == p.spaceHeader && res.Payload.SpaceSettingsPayloadId == p.settingsId && bytes.Equal(p.credential, res.Credential)
}

func (p pushSpaceRequestMatcher) String() string {
	return ""
}

func (fx *headSyncFixture) initDiffSyncer(t *testing.T) {
	fx.init(t)
	fx.diffSyncer = newDiffSyncer(fx.headSync).(*diffSyncer)
	fx.diffSyncer.clientFactory = spacesyncproto.ClientFactoryFunc(func(cc drpc.Conn) spacesyncproto.DRPCSpaceSyncClient {
		return fx.clientMock
	})
	fx.deletionStateMock.EXPECT().AddObserver(gomock.Any())
	fx.diffSyncer.Init()
}

func TestDiffSyncer(t *testing.T) {
	ctx := context.Background()

	t.Run("diff syncer sync", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		mPeer := rpctest.MockPeer{}
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.treeSyncerMock.EXPECT().ShouldSync(gomock.Any()).Return(true)
		fx.peerManagerMock.EXPECT().
			GetResponsiblePeers(gomock.Any()).
			Return([]peer.Peer{mPeer}, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(NewRemoteDiff(fx.spaceState.SpaceId, fx.clientMock))).
			Return([]string{"new"}, []string{"changed"}, nil, nil)
		fx.deletionStateMock.EXPECT().Filter([]string{"new"}).Return([]string{"new"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter([]string{"changed"}).Return([]string{"changed"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter(nil).Return(nil).Times(1)
		fx.treeSyncerMock.EXPECT().SyncAll(gomock.Any(), mPeer, []string{"changed"}, []string{"new"}).Return(nil)
		require.NoError(t, fx.diffSyncer.Sync(ctx))
	})

	t.Run("diff syncer sync, acl changed", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		mPeer := rpctest.MockPeer{}
		remDiff := NewRemoteDiff(fx.spaceState.SpaceId, fx.clientMock)
		fx.treeSyncerMock.EXPECT().ShouldSync(gomock.Any()).Return(true)
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.peerManagerMock.EXPECT().
			GetResponsiblePeers(gomock.Any()).
			Return([]peer.Peer{mPeer}, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remDiff)).
			Return([]string{"new"}, []string{"changed"}, nil, nil)
		fx.deletionStateMock.EXPECT().Filter([]string{"new"}).Return([]string{"new"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter([]string{"changed"}).Return([]string{"changed", "aclId"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter(nil).Return(nil).Times(1)
		fx.treeSyncerMock.EXPECT().SyncAll(gomock.Any(), mPeer, []string{"changed"}, []string{"new"}).Return(nil)
		fx.aclMock.EXPECT().SyncWithPeer(gomock.Any(), mPeer).Return(nil)
		require.NoError(t, fx.diffSyncer.Sync(ctx))
	})

	t.Run("diff syncer sync conf error", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		ctx := context.Background()
		fx.peerManagerMock.EXPECT().
			GetResponsiblePeers(gomock.Any()).
			Return(nil, fmt.Errorf("some error"))

		require.Error(t, fx.diffSyncer.Sync(ctx))
	})

	t.Run("deletion state remove objects", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		deletedId := "id"
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.deletionStateMock.EXPECT().Exists(deletedId).Return(true)

		// this should not result in any mock being called
		fx.diffSyncer.UpdateHeads(deletedId, []string{"someHead"})
	})

	t.Run("update heads updates diff", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		newId := "newId"
		newHeads := []string{"h1", "h2"}
		hash := "hash"
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.diffMock.EXPECT().Set(ldiff.Element{
			Id:   newId,
			Head: concatStrings(newHeads),
		})
		fx.diffMock.EXPECT().Hash().Return(hash)
		fx.deletionStateMock.EXPECT().Exists(newId).Return(false)
		fx.storageMock.EXPECT().WriteSpaceHash(hash)
		fx.diffSyncer.UpdateHeads(newId, newHeads)
	})

	t.Run("diff syncer sync space missing", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.treeSyncerMock.EXPECT().ShouldSync(gomock.Any()).Return(true)
		aclStorageMock := mock_liststorage.NewMockListStorage(fx.ctrl)
		settingsStorage := mock_treestorage.NewMockTreeStorage(fx.ctrl)
		settingsId := "settingsId"
		aclRootId := "aclRootId"
		aclRoot := &consensusproto.RawRecordWithId{
			Id: aclRootId,
		}
		settingsRoot := &treechangeproto.RawTreeChangeWithId{
			Id: settingsId,
		}
		spaceHeader := &spacesyncproto.RawSpaceHeaderWithId{}
		spaceSettingsId := "spaceSettingsId"
		credential := []byte("credential")
		remDiff := NewRemoteDiff(fx.spaceState.SpaceId, fx.clientMock)

		fx.peerManagerMock.EXPECT().
			GetResponsiblePeers(gomock.Any()).
			Return([]peer.Peer{rpctest.MockPeer{}}, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remDiff)).
			Return(nil, nil, nil, spacesyncproto.ErrSpaceMissing)

		fx.storageMock.EXPECT().AclStorage().Return(aclStorageMock, nil)
		fx.storageMock.EXPECT().SpaceHeader().Return(spaceHeader, nil)
		fx.storageMock.EXPECT().SpaceSettingsId().Return(spaceSettingsId)
		fx.storageMock.EXPECT().TreeStorage(spaceSettingsId).Return(settingsStorage, nil)

		settingsStorage.EXPECT().Root().Return(settingsRoot, nil)
		aclStorageMock.EXPECT().
			Root().
			Return(aclRoot, nil)
		fx.credentialProviderMock.EXPECT().
			GetCredential(gomock.Any(), spaceHeader).
			Return(credential, nil)
		fx.clientMock.EXPECT().
			SpacePush(gomock.Any(), newPushSpaceRequestMatcher(fx.spaceState.SpaceId, aclRootId, settingsId, credential, spaceHeader)).
			Return(nil, nil)
		fx.peerManagerMock.EXPECT().SendMessage(gomock.Any(), "peerId", gomock.Any()).Return(nil)
		require.NoError(t, fx.diffSyncer.Sync(ctx))
	})

	t.Run("diff syncer sync unexpected", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		remDiff := NewRemoteDiff(fx.spaceState.SpaceId, fx.clientMock)
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.treeSyncerMock.EXPECT().ShouldSync(gomock.Any()).Return(true)
		fx.peerManagerMock.EXPECT().
			GetResponsiblePeers(gomock.Any()).
			Return([]peer.Peer{rpctest.MockPeer{}}, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remDiff)).
			Return(nil, nil, nil, spacesyncproto.ErrUnexpected)

		require.NoError(t, fx.diffSyncer.Sync(ctx))
	})

	t.Run("diff syncer sync space is deleted error", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		mPeer := rpctest.MockPeer{}
		remDiff := NewRemoteDiff(fx.spaceState.SpaceId, fx.clientMock)
		fx.treeSyncerMock.EXPECT().ShouldSync(gomock.Any()).Return(true)
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.peerManagerMock.EXPECT().
			GetResponsiblePeers(gomock.Any()).
			Return([]peer.Peer{mPeer}, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remDiff)).
			Return(nil, nil, nil, spacesyncproto.ErrSpaceIsDeleted)

		require.NoError(t, fx.diffSyncer.Sync(ctx))
	})
}
