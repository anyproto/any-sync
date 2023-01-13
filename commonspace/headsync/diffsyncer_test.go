package headsync

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/app/ldiff"
	"github.com/anytypeio/any-sync/app/ldiff/mock_ldiff"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/confconnector/mock_confconnector"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	mock_treestorage "github.com/anytypeio/any-sync/commonspace/object/tree/treestorage/mock_treestorage"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter/mock_treegetter"
	"github.com/anytypeio/any-sync/commonspace/settings/deletionstate/mock_deletionstate"
	"github.com/anytypeio/any-sync/commonspace/spacestorage/mock_spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto/mock_spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p/core/sec"
	"github.com/stretchr/testify/require"
	"storj.io/drpc"
	"testing"
	"time"
)

type pushSpaceRequestMatcher struct {
	spaceId     string
	aclRootId   string
	settingsId  string
	spaceHeader *spacesyncproto.RawSpaceHeaderWithId
}

func (p pushSpaceRequestMatcher) Matches(x interface{}) bool {
	res, ok := x.(*spacesyncproto.SpacePushRequest)
	if !ok {
		return false
	}

	return res.Payload.AclPayloadId == p.aclRootId && res.Payload.SpaceHeader == p.spaceHeader && res.Payload.SpaceSettingsPayloadId == p.settingsId
}

func (p pushSpaceRequestMatcher) String() string {
	return ""
}

type mockPeer struct{}

func (m mockPeer) Id() string {
	return "mockId"
}

func (m mockPeer) LastUsage() time.Time {
	return time.Time{}
}

func (m mockPeer) Secure() sec.SecureConn {
	return nil
}

func (m mockPeer) UpdateLastUsage() {
}

func (m mockPeer) Close() error {
	return nil
}

func (m mockPeer) Closed() <-chan struct{} {
	return make(chan struct{})
}

func (m mockPeer) Invoke(ctx context.Context, rpc string, enc drpc.Encoding, in, out drpc.Message) error {
	return nil
}

func (m mockPeer) NewStream(ctx context.Context, rpc string, enc drpc.Encoding) (drpc.Stream, error) {
	return nil, nil
}

func newPushSpaceRequestMatcher(
	spaceId string,
	aclRootId string,
	settingsId string,
	spaceHeader *spacesyncproto.RawSpaceHeaderWithId) *pushSpaceRequestMatcher {
	return &pushSpaceRequestMatcher{
		spaceId:     spaceId,
		aclRootId:   aclRootId,
		settingsId:  settingsId,
		spaceHeader: spaceHeader,
	}
}

func TestDiffSyncer_Sync(t *testing.T) {
	// setup
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	diffMock := mock_ldiff.NewMockDiff(ctrl)
	connectorMock := mock_confconnector.NewMockConfConnector(ctrl)
	cacheMock := mock_treegetter.NewMockTreeGetter(ctrl)
	stMock := mock_spacestorage.NewMockSpaceStorage(ctrl)
	clientMock := mock_spacesyncproto.NewMockDRPCSpaceSyncClient(ctrl)
	factory := spacesyncproto.ClientFactoryFunc(func(cc drpc.Conn) spacesyncproto.DRPCSpaceSyncClient {
		return clientMock
	})
	delState := mock_deletionstate.NewMockDeletionState(ctrl)
	spaceId := "spaceId"
	aclRootId := "aclRootId"
	l := logger.NewNamed(spaceId)
	diffSyncer := newDiffSyncer(spaceId, diffMock, connectorMock, cacheMock, stMock, factory, syncstatus.NewNoOpSyncStatus(), l)
	delState.EXPECT().AddObserver(gomock.Any())
	diffSyncer.Init(delState)

	t.Run("diff syncer sync", func(t *testing.T) {
		connectorMock.EXPECT().
			GetResponsiblePeers(gomock.Any(), spaceId).
			Return([]peer.Peer{mockPeer{}}, nil)
		diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(NewRemoteDiff(spaceId, clientMock))).
			Return([]string{"new"}, []string{"changed"}, nil, nil)
		delState.EXPECT().FilterJoin(gomock.Any()).Return([]string{"new", "changed"})
		for _, arg := range []string{"new", "changed"} {
			cacheMock.EXPECT().
				GetTree(gomock.Any(), spaceId, arg).
				Return(nil, nil)
		}
		require.NoError(t, diffSyncer.Sync(ctx))
	})

	t.Run("diff syncer sync conf error", func(t *testing.T) {
		connectorMock.EXPECT().
			GetResponsiblePeers(gomock.Any(), spaceId).
			Return(nil, fmt.Errorf("some error"))

		require.Error(t, diffSyncer.Sync(ctx))
	})

	t.Run("deletion state remove objects", func(t *testing.T) {
		deletedId := "id"
		delState.EXPECT().Exists(deletedId).Return(true)

		// this should not result in any mock being called
		diffSyncer.UpdateHeads(deletedId, []string{"someHead"})
	})

	t.Run("update heads updates diff", func(t *testing.T) {
		newId := "newId"
		newHeads := []string{"h1", "h2"}
		hash := "hash"
		diffMock.EXPECT().Set(ldiff.Element{
			Id:   newId,
			Head: concatStrings(newHeads),
		})
		diffMock.EXPECT().Hash().Return(hash)
		delState.EXPECT().Exists(newId).Return(false)
		stMock.EXPECT().WriteSpaceHash(hash)
		diffSyncer.UpdateHeads(newId, newHeads)
	})

	t.Run("diff syncer sync space missing", func(t *testing.T) {
		aclStorageMock := mock_treestorage.NewMockListStorage(ctrl)
		settingsStorage := mock_treestorage.NewMockTreeStorage(ctrl)
		settingsId := "settingsId"
		aclRoot := &aclrecordproto.RawAclRecordWithId{
			Id: aclRootId,
		}
		settingsRoot := &treechangeproto.RawTreeChangeWithId{
			Id: settingsId,
		}
		spaceHeader := &spacesyncproto.RawSpaceHeaderWithId{}
		spaceSettingsId := "spaceSettingsId"

		connectorMock.EXPECT().
			GetResponsiblePeers(gomock.Any(), spaceId).
			Return([]peer.Peer{mockPeer{}}, nil)
		diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(NewRemoteDiff(spaceId, clientMock))).
			Return(nil, nil, nil, spacesyncproto.ErrSpaceMissing)

		stMock.EXPECT().AclStorage().Return(aclStorageMock, nil)
		stMock.EXPECT().SpaceHeader().Return(spaceHeader, nil)
		stMock.EXPECT().SpaceSettingsId().Return(spaceSettingsId)
		stMock.EXPECT().TreeStorage(spaceSettingsId).Return(settingsStorage, nil)

		settingsStorage.EXPECT().Root().Return(settingsRoot, nil)
		aclStorageMock.EXPECT().
			Root().
			Return(aclRoot, nil)
		clientMock.EXPECT().
			SpacePush(gomock.Any(), newPushSpaceRequestMatcher(spaceId, aclRootId, settingsId, spaceHeader)).
			Return(nil, nil)

		require.NoError(t, diffSyncer.Sync(ctx))
	})

	t.Run("diff syncer sync other error", func(t *testing.T) {
		connectorMock.EXPECT().
			GetResponsiblePeers(gomock.Any(), spaceId).
			Return([]peer.Peer{mockPeer{}}, nil)
		diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(NewRemoteDiff(spaceId, clientMock))).
			Return(nil, nil, nil, spacesyncproto.ErrUnexpected)

		require.NoError(t, diffSyncer.Sync(ctx))
	})
}
