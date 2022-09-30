package diffservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/remotediff"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
	storage2 "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ldiff"
	"github.com/golang/mock/gomock"
	"storj.io/drpc"
	"testing"
)

type pushSpaceRequestMatcher struct {
	spaceId     string
	aclRoot     *aclrecordproto.RawACLRecordWithId
	spaceHeader *spacesyncproto.SpaceHeader
}

func (p pushSpaceRequestMatcher) Matches(x interface{}) bool {
	res, ok := x.(*spacesyncproto.PushSpaceRequest)
	if !ok {
		return false
	}

	return res.SpaceId == p.spaceId && res.AclRoot == p.aclRoot && res.SpaceHeader == p.spaceHeader
}

func (p pushSpaceRequestMatcher) String() string {
	return ""
}

func newPushSpaceRequestMatcher(
	spaceId string,
	aclRoot *aclrecordproto.RawACLRecordWithId,
	spaceHeader *spacesyncproto.SpaceHeader) *pushSpaceRequestMatcher {
	return &pushSpaceRequestMatcher{
		spaceId:     spaceId,
		aclRoot:     aclRoot,
		spaceHeader: spaceHeader,
	}
}

func TestDiffSyncer_Sync(t *testing.T) {
	// setup
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	diffMock := ldiff.NewMockDiff(ctrl)
	nconfMock := nodeconf.NewMockConfiguration(ctrl)
	cacheMock := cache.NewMockTreeCache(ctrl)
	stMock := storage.NewMockSpaceStorage(ctrl)
	clientMock := spacesyncproto.NewMockDRPCSpaceClient(ctrl)
	factory := spacesyncproto.ClientFactoryFunc(func(cc drpc.Conn) spacesyncproto.DRPCSpaceClient {
		return clientMock
	})
	spaceId := "spaceId"
	l := logger.NewNamed(spaceId)
	diffSyncer := newDiffSyncer(spaceId, diffMock, nconfMock, cacheMock, stMock, factory, l)

	t.Run("diff syncer sync simple", func(t *testing.T) {
		nconfMock.EXPECT().
			ResponsiblePeers(gomock.Any(), spaceId).
			Return([]peer.Peer{nil}, nil)
		diffMock.EXPECT().
			Diff(gomock.Any(), gomock.Eq(remotediff.NewRemoteDiff(spaceId, clientMock))).
			Return([]string{"new"}, []string{"changed"}, nil, nil)
		for _, arg := range []string{"new", "changed"} {
			cacheMock.EXPECT().
				GetTree(gomock.Any(), spaceId, arg).
				Return(cache.TreeResult{}, nil)
		}
		_ = diffSyncer.Sync(ctx)
	})

	t.Run("diff syncer sync space missing", func(t *testing.T) {
		aclStorageMock := storage2.NewMockListStorage(ctrl)
		aclRoot := &aclrecordproto.RawACLRecordWithId{}
		spaceHeader := &spacesyncproto.SpaceHeader{}

		nconfMock.EXPECT().
			ResponsiblePeers(gomock.Any(), spaceId).
			Return([]peer.Peer{nil}, nil)
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
			PushSpace(gomock.Any(), newPushSpaceRequestMatcher(spaceId, aclRoot, spaceHeader)).
			Return(nil, nil)

		_ = diffSyncer.Sync(ctx)
	})
}
