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
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/statestorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/list/mock_list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
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

	return res.Payload.AclPayloadId == p.aclRootId && bytes.Equal(res.Payload.SpaceHeader.RawHeader, p.spaceHeader.RawHeader) && res.Payload.SpaceSettingsPayloadId == p.settingsId && bytes.Equal(p.credential, res.Credential)
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
		fx.diffContainerMock.EXPECT().DiffTypeCheck(gomock.Any(), gomock.Any()).Return(true, fx.diffMock, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(NewRemoteDiff(fx.spaceState.SpaceId, fx.clientMock))).
			Return([]string{"new"}, []string{"changed"}, nil, nil)
		fx.deletionStateMock.EXPECT().Filter([]string{"new"}).Return([]string{"new"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter([]string{"changed"}).Return([]string{"changed"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter(nil).Return(nil).Times(1)
		fx.treeSyncerMock.EXPECT().SyncAll(gomock.Any(), mPeer, []string{"changed"}, []string{"new"}).Return(nil)
		fx.peerManagerMock.EXPECT().KeepAlive(gomock.Any())

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
		fx.diffContainerMock.EXPECT().DiffTypeCheck(gomock.Any(), gomock.Any()).Return(true, fx.diffMock, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remDiff)).
			Return([]string{"new"}, []string{"changed"}, nil, nil)
		fx.deletionStateMock.EXPECT().Filter([]string{"new"}).Return([]string{"new"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter([]string{"changed"}).Return([]string{"changed", "aclId"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter(nil).Return(nil).Times(1)
		fx.treeSyncerMock.EXPECT().SyncAll(gomock.Any(), mPeer, []string{"changed"}, []string{"new"}).Return(nil)
		fx.aclMock.EXPECT().SyncWithPeer(gomock.Any(), mPeer).Return(nil)
		fx.peerManagerMock.EXPECT().KeepAlive(gomock.Any())

		require.NoError(t, fx.diffSyncer.Sync(ctx))
	})

	t.Run("diff syncer sync, store changed", func(t *testing.T) {
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
		fx.diffContainerMock.EXPECT().DiffTypeCheck(gomock.Any(), gomock.Any()).Return(true, fx.diffMock, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remDiff)).
			Return([]string{"new"}, []string{"changed"}, nil, nil)
		fx.deletionStateMock.EXPECT().Filter([]string{"new"}).Return([]string{"new"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter([]string{"changed"}).Return([]string{"changed", "store"}).Times(1)
		fx.deletionStateMock.EXPECT().Filter(nil).Return(nil).Times(1)
		fx.treeSyncerMock.EXPECT().SyncAll(gomock.Any(), mPeer, []string{"changed"}, []string{"new"}).Return(nil)
		fx.kvMock.EXPECT().SyncWithPeer(mPeer).Return(nil)
		fx.peerManagerMock.EXPECT().KeepAlive(gomock.Any())

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
		fx.diffContainerMock.EXPECT().RemoveId(deletedId).Return(nil)
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().AnyTimes().Return("hash")
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash", "hash").Return(nil)

		upd := headstorage.DeletedStatusDeleted
		fx.diffSyncer.updateHeads(headstorage.HeadsUpdate{
			Id:            "id",
			DeletedStatus: &upd,
		})
	})

	t.Run("deletion state update objects", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		updatedId := "id"
		fx.diffMock.EXPECT().Hash().Return("hash")
		fx.deletionStateMock.EXPECT().Exists(updatedId).Return(false)
		fx.diffContainerMock.EXPECT().Set(ldiff.Element{
			Id:   updatedId,
			Head: "head",
		})
		fx.diffContainerMock.EXPECT().NewDiff().Return(fx.diffMock)
		fx.diffContainerMock.EXPECT().OldDiff().Return(fx.diffMock)
		fx.diffMock.EXPECT().Hash().AnyTimes().Return("hash")
		fx.stateStorage.EXPECT().SetHash(gomock.Any(), "hash", "hash").Return(nil)
		fx.diffSyncer.updateHeads(headstorage.HeadsUpdate{
			Id:    "id",
			Heads: []string{"head"},
		})
	})

	t.Run("diff syncer sync space missing", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.treeSyncerMock.EXPECT().ShouldSync(gomock.Any()).Return(true)
		aclStorageMock := mock_list.NewMockStorage(fx.ctrl)
		settingsStorage := mock_objecttree.NewMockStorage(fx.ctrl)
		settingsId := "settingsId"
		aclRootId := "aclRootId"
		spaceHeader := &spacesyncproto.RawSpaceHeaderWithId{
			RawHeader: []byte{1},
		}
		credential := []byte("credential")
		remDiff := NewRemoteDiff(fx.spaceState.SpaceId, fx.clientMock)

		fx.peerManagerMock.EXPECT().
			GetResponsiblePeers(gomock.Any()).
			Return([]peer.Peer{rpctest.MockPeer{}}, nil)
		fx.diffContainerMock.EXPECT().DiffTypeCheck(gomock.Any(), gomock.Any()).Return(true, fx.diffMock, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remDiff)).
			Return(nil, nil, nil, spacesyncproto.ErrSpaceMissing)

		fx.storageMock.EXPECT().AclStorage().Return(aclStorageMock, nil)
		fx.stateStorage.EXPECT().GetState(gomock.Any()).Return(statestorage.State{
			SettingsId:  settingsId,
			SpaceHeader: []byte{1},
		}, nil)
		fx.storageMock.EXPECT().TreeStorage(gomock.Any(), settingsId).Return(settingsStorage, nil)

		settingsStorage.EXPECT().Root(gomock.Any()).Return(objecttree.StorageChange{RawChange: nil, Id: settingsId}, nil)
		aclStorageMock.EXPECT().
			Root(gomock.Any()).
			Return(list.StorageRecord{
				Id: aclRootId,
			}, nil)
		fx.credentialProviderMock.EXPECT().
			GetCredential(gomock.Any(), spaceHeader).
			Return(credential, nil)
		fx.clientMock.EXPECT().
			SpacePush(gomock.Any(), newPushSpaceRequestMatcher(fx.spaceState.SpaceId, aclRootId, settingsId, credential, spaceHeader)).
			Return(nil, nil)
		fx.peerManagerMock.EXPECT().SendMessage(gomock.Any(), "peerId", gomock.Any()).Return(nil)
		fx.peerManagerMock.EXPECT().KeepAlive(gomock.Any())

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

		fx.diffContainerMock.EXPECT().DiffTypeCheck(gomock.Any(), gomock.Any()).Return(true, fx.diffMock, nil)
		fx.diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remDiff)).
			Return(nil, nil, nil, spacesyncproto.ErrUnexpected)
		fx.peerManagerMock.EXPECT().KeepAlive(gomock.Any())

		require.NoError(t, fx.diffSyncer.Sync(ctx))
	})

	t.Run("diff syncer sync space is deleted error", func(t *testing.T) {
		fx := newHeadSyncFixture(t)
		fx.initDiffSyncer(t)
		defer fx.stop()
		mPeer := rpctest.MockPeer{}
		fx.treeSyncerMock.EXPECT().ShouldSync(gomock.Any()).Return(true)
		fx.aclMock.EXPECT().Id().AnyTimes().Return("aclId")
		fx.peerManagerMock.EXPECT().
			GetResponsiblePeers(gomock.Any()).
			Return([]peer.Peer{mPeer}, nil)
		fx.diffContainerMock.EXPECT().DiffTypeCheck(gomock.Any(), gomock.Any()).Return(true, fx.diffMock, spacesyncproto.ErrSpaceIsDeleted)
		fx.peerManagerMock.EXPECT().KeepAlive(gomock.Any())

		require.NoError(t, fx.diffSyncer.Sync(ctx))
	})
}
