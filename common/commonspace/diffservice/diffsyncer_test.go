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
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/ldiff"
	"github.com/golang/mock/gomock"
	"storj.io/drpc"
	"testing"
)

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
		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, "new").
			Return(cache.TreeResult{}, nil)
		cacheMock.EXPECT().
			GetTree(gomock.Any(), spaceId, "changed").
			Return(cache.TreeResult{}, nil)
		_ = diffSyncer.Sync(ctx)
	})
}
