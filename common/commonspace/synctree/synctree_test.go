package synctree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage/mock_storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/mock_synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener/mock_updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/list/mock_list"
	storage2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	tree "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree/mock_objecttree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

type syncTreeMatcher struct {
	objTree  tree.ObjectTree
	client   SyncClient
	listener updatelistener.UpdateListener
}

func (s syncTreeMatcher) Matches(x interface{}) bool {
	t, ok := x.(*syncTree)
	if !ok {
		return false
	}
	return s.objTree == t.ObjectTree && t.syncClient == s.client && t.listener == s.listener
}

func (s syncTreeMatcher) String() string {
	return ""
}

func Test_DeriveSyncTree(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	aclListMock := mock_list.NewMockACLList(ctrl)
	objTreeMock := newTestObjMock(mock_tree.NewMockObjectTree(ctrl))
	spaceStorageMock := mock_storage.NewMockSpaceStorage(ctrl)
	spaceId := "spaceId"
	expectedPayload := tree.ObjectTreeCreatePayload{SpaceId: spaceId}
	createDerivedObjectTree = func(payload tree.ObjectTreeCreatePayload, l list.ACLList, create storage2.TreeStorageCreatorFunc) (objTree tree.ObjectTree, err error) {
		require.Equal(t, l, aclListMock)
		require.Equal(t, expectedPayload, payload)
		return objTreeMock, nil
	}
	createSyncClient = func(spaceId string, pool syncservice.StreamPool, factory RequestFactory, configuration nodeconf.Configuration, checker syncservice.StreamChecker) SyncClient {
		return syncClientMock
	}
	headUpdate := &treechangeproto.TreeSyncMessage{}
	syncClientMock.EXPECT().CreateHeadUpdate(gomock.Any(), gomock.Nil()).Return(headUpdate)
	syncClientMock.EXPECT().BroadcastAsync(gomock.Eq(headUpdate)).Return(nil)
	deps := CreateDeps{
		AclList:      aclListMock,
		SpaceId:      spaceId,
		Payload:      expectedPayload,
		SpaceStorage: spaceStorageMock,
	}

	_, err := DeriveSyncTree(ctx, deps)
	require.NoError(t, err)
}

func Test_CreateSyncTree(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	aclListMock := mock_list.NewMockACLList(ctrl)
	objTreeMock := newTestObjMock(mock_tree.NewMockObjectTree(ctrl))
	spaceStorageMock := mock_storage.NewMockSpaceStorage(ctrl)
	spaceId := "spaceId"
	expectedPayload := tree.ObjectTreeCreatePayload{SpaceId: spaceId}
	createObjectTree = func(payload tree.ObjectTreeCreatePayload, l list.ACLList, create storage2.TreeStorageCreatorFunc) (objTree tree.ObjectTree, err error) {
		require.Equal(t, l, aclListMock)
		require.Equal(t, expectedPayload, payload)
		return objTreeMock, nil
	}
	createSyncClient = func(spaceId string, pool syncservice.StreamPool, factory RequestFactory, configuration nodeconf.Configuration, checker syncservice.StreamChecker) SyncClient {
		return syncClientMock
	}
	headUpdate := &treechangeproto.TreeSyncMessage{}
	syncClientMock.EXPECT().CreateHeadUpdate(gomock.Any(), gomock.Nil()).Return(headUpdate)
	syncClientMock.EXPECT().BroadcastAsync(gomock.Eq(headUpdate)).Return(nil)
	deps := CreateDeps{
		AclList:      aclListMock,
		SpaceId:      spaceId,
		Payload:      expectedPayload,
		SpaceStorage: spaceStorageMock,
	}

	_, err := CreateSyncTree(ctx, deps)
	require.NoError(t, err)
}

func Test_BuildSyncTree(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	updateListenerMock := mock_updatelistener.NewMockUpdateListener(ctrl)
	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	objTreeMock := newTestObjMock(mock_tree.NewMockObjectTree(ctrl))
	tr := &syncTree{
		ObjectTree:  objTreeMock,
		SyncHandler: nil,
		syncClient:  syncClientMock,
		listener:    updateListenerMock,
		isClosed:    false,
	}

	headUpdate := &treechangeproto.TreeSyncMessage{}
	t.Run("AddRawChanges update", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		expectedRes := tree.AddResult{
			Added: changes,
			Mode:  tree.Append,
		}
		objTreeMock.EXPECT().AddRawChanges(gomock.Any(), gomock.Eq(changes)).
			Return(expectedRes, nil)
		updateListenerMock.EXPECT().Update(tr)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), gomock.Eq(changes)).Return(headUpdate)
		syncClientMock.EXPECT().BroadcastAsync(gomock.Eq(headUpdate)).Return(nil)
		res, err := tr.AddRawChanges(ctx, changes...)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})

	t.Run("AddRawChanges rebuild", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		expectedRes := tree.AddResult{
			Added: changes,
			Mode:  tree.Rebuild,
		}
		objTreeMock.EXPECT().AddRawChanges(gomock.Any(), gomock.Eq(changes)).
			Return(expectedRes, nil)
		updateListenerMock.EXPECT().Rebuild(tr)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), gomock.Eq(changes)).Return(headUpdate)
		syncClientMock.EXPECT().BroadcastAsync(gomock.Eq(headUpdate)).Return(nil)
		res, err := tr.AddRawChanges(ctx, changes...)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})

	t.Run("AddRawChanges nothing", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		expectedRes := tree.AddResult{
			Added: changes,
			Mode:  tree.Nothing,
		}
		objTreeMock.EXPECT().AddRawChanges(gomock.Any(), gomock.Eq(changes)).
			Return(expectedRes, nil)

		res, err := tr.AddRawChanges(ctx, changes...)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})

	t.Run("AddContent", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		content := tree.SignableChangeContent{
			Data: []byte("abcde"),
		}
		expectedRes := tree.AddResult{
			Mode:  tree.Append,
			Added: changes,
		}
		objTreeMock.EXPECT().AddContent(gomock.Any(), gomock.Eq(content)).
			Return(expectedRes, nil)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), gomock.Eq(changes)).Return(headUpdate)
		syncClientMock.EXPECT().BroadcastAsync(gomock.Eq(headUpdate)).Return(nil)
		res, err := tr.AddContent(ctx, content)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})
}
