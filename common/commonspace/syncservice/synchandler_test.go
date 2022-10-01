package syncservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache/mock_cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/mock_syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	mock_tree "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree/mock_objecttree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treechangeproto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

type treeContainer struct {
	objTree tree.ObjectTree
}

func (t treeContainer) Tree() tree.ObjectTree {
	return t.objTree
}

func TestSyncHandler_HandleMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	spaceId := "spaceId"
	cacheMock := mock_cache.NewMockTreeCache(ctrl)
	syncClientMock := mock_syncservice.NewMockSyncClient(ctrl)
	objectTreeMock := mock_tree.NewMockObjectTree(ctrl)

	syncHandler := newSyncHandler(spaceId, cacheMock, syncClientMock)
	treeId := "treeId"
	senderId := "senderId"
	chWithId := &treechangeproto.RawTreeChangeWithId{}
	headUpdate := &spacesyncproto.ObjectHeadUpdate{
		Heads:        []string{"h1"},
		Changes:      []*treechangeproto.RawTreeChangeWithId{chWithId},
		SnapshotPath: []string{"h1"},
	}
	msg := spacesyncproto.WrapHeadUpdate(headUpdate, chWithId, treeId, "")
	cacheMock.EXPECT().
		GetTree(gomock.Any(), spaceId, treeId).
		Return(cache.TreeResult{
			Release:       func() {},
			TreeContainer: treeContainer{objectTreeMock},
		}, nil)
	objectTreeMock.EXPECT().Lock()
	objectTreeMock.EXPECT().Heads().Return([]string{"h2"})
	objectTreeMock.EXPECT().AddRawChanges(gomock.Any(), gomock.Eq([]*treechangeproto.RawTreeChangeWithId{chWithId})).
		Return(tree.AddResult{}, nil)
	objectTreeMock.EXPECT().Unlock()
	err := syncHandler.HandleMessage(ctx, senderId, msg)
	require.NoError(t, err)
}
