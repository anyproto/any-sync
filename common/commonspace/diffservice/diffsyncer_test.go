package diffservice

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/remotediff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto/mock_spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage/mock_storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter/mock_treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf/mock_nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	mock_aclstorage "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage/mock_storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/ldiff/mock_ldiff"
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
	spaceHeader *spacesyncproto.RawSpaceHeaderWithId
}

func (p pushSpaceRequestMatcher) Matches(x interface{}) bool {
	res, ok := x.(*spacesyncproto.PushSpaceRequest)
	if !ok {
		return false
	}

	return res.Payload.AclPayloadId == p.aclRootId && res.Payload.SpaceHeader == p.spaceHeader
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
	spaceHeader *spacesyncproto.RawSpaceHeaderWithId) *pushSpaceRequestMatcher {
	return &pushSpaceRequestMatcher{
		spaceId:     spaceId,
		aclRootId:   aclRootId,
		spaceHeader: spaceHeader,
	}
}

func TestDiffSyncer_Sync(t *testing.T) {
	// setup
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	diffMock := mock_ldiff.NewMockDiff(ctrl)
	connectorMock := mock_nodeconf.NewMockConfConnector(ctrl)
	cacheMock := mock_treegetter.NewMockTreeGetter(ctrl)
	stMock := mock_storage.NewMockSpaceStorage(ctrl)
	clientMock := mock_spacesyncproto.NewMockDRPCSpaceClient(ctrl)
	factory := spacesyncproto.ClientFactoryFunc(func(cc drpc.Conn) spacesyncproto.DRPCSpaceClient {
		return clientMock
	})
	spaceId := "spaceId"
	aclRootId := "aclRootId"
	l := logger.NewNamed(spaceId)
	diffSyncer := newDiffSyncer(spaceId, diffMock, connectorMock, cacheMock, stMock, factory, l)

	t.Run("diff syncer sync simple", func(t *testing.T) {
		connectorMock.EXPECT().
			GetResponsiblePeers(gomock.Any(), spaceId).
			Return([]peer.Peer{mockPeer{}}, nil)
		diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remotediff.NewRemoteDiff(spaceId, clientMock))).
			Return([]string{"new"}, []string{"changed"}, nil, nil)
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

	t.Run("diff syncer sync space missing", func(t *testing.T) {
		aclStorageMock := mock_aclstorage.NewMockListStorage(ctrl)
		aclRoot := &aclrecordproto.RawACLRecordWithId{
			Id: aclRootId,
		}
		spaceHeader := &spacesyncproto.RawSpaceHeaderWithId{}

		connectorMock.EXPECT().
			GetResponsiblePeers(gomock.Any(), spaceId).
			Return([]peer.Peer{mockPeer{}}, nil)
		diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remotediff.NewRemoteDiff(spaceId, clientMock))).
			Return(nil, nil, nil, spacesyncproto.ErrSpaceMissing)
		stMock.EXPECT().
			ACLStorage().
			Return(aclStorageMock, nil)
		stMock.EXPECT().
			SpaceHeader().
			Return(spaceHeader, nil)
		aclStorageMock.EXPECT().
			Root().
			Return(aclRoot, nil)
		clientMock.EXPECT().
			PushSpace(gomock.Any(), newPushSpaceRequestMatcher(spaceId, aclRootId, spaceHeader)).
			Return(nil, nil)

		require.NoError(t, diffSyncer.Sync(ctx))
	})

	t.Run("diff syncer sync other error", func(t *testing.T) {
		connectorMock.EXPECT().
			GetResponsiblePeers(gomock.Any(), spaceId).
			Return([]peer.Peer{mockPeer{}}, nil)
		diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remotediff.NewRemoteDiff(spaceId, clientMock))).
			Return(nil, nil, nil, spacesyncproto.ErrUnexpected)

		require.NoError(t, diffSyncer.Sync(ctx))
	})
}
