package synctree

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/list/mock_list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/objecttree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/synctree/updatelistener/mock_updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectsync"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacestorage/mock_spacestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncstatus"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

type syncTreeMatcher struct {
	objTree  objecttree.ObjectTree
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

func syncClientFuncCreator(client SyncClient) func(spaceId string, factory RequestFactory, objectSync objectsync.ObjectSync, configuration nodeconf.Configuration) SyncClient {
	return func(spaceId string, factory RequestFactory, objectSync objectsync.ObjectSync, configuration nodeconf.Configuration) SyncClient {
		return client
	}
}

func Test_DeriveSyncTree(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	aclListMock := mock_list.NewMockAclList(ctrl)
	objTreeMock := newTestObjMock(mock_objecttree.NewMockObjectTree(ctrl))
	spaceStorageMock := mock_spacestorage.NewMockSpaceStorage(ctrl)
	headNotifiableMock := mock_synctree.NewMockHeadNotifiable(ctrl)
	spaceId := "spaceId"
	expectedPayload := objecttree.ObjectTreeCreatePayload{SpaceId: spaceId}
	createDerivedObjectTree = func(payload objecttree.ObjectTreeCreatePayload, l list.AclList, create treestorage.TreeStorageCreatorFunc) (objTree objecttree.ObjectTree, err error) {
		require.Equal(t, l, aclListMock)
		require.Equal(t, expectedPayload, payload)
		return objTreeMock, nil
	}
	createSyncClient = syncClientFuncCreator(syncClientMock)
	headUpdate := &treechangeproto.TreeSyncMessage{}
	objTreeMock.EXPECT().Heads().AnyTimes().Return([]string{"h1"})
	headNotifiableMock.EXPECT().UpdateHeads("id", []string{"h1"})
	syncClientMock.EXPECT().CreateHeadUpdate(gomock.Any(), gomock.Nil()).Return(headUpdate)
	syncClientMock.EXPECT().BroadcastAsync(gomock.Eq(headUpdate)).Return(nil)
	deps := CreateDeps{
		AclList:        aclListMock,
		SpaceId:        spaceId,
		Payload:        expectedPayload,
		SpaceStorage:   spaceStorageMock,
		SyncStatus:     syncstatus.NewNoOpSyncStatus(),
		HeadNotifiable: headNotifiableMock,
	}
	objTreeMock.EXPECT().Id().Return("id")

	_, err := DeriveSyncTree(ctx, deps)
	require.NoError(t, err)
}

func Test_CreateSyncTree(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	aclListMock := mock_list.NewMockAclList(ctrl)
	objTreeMock := newTestObjMock(mock_objecttree.NewMockObjectTree(ctrl))
	spaceStorageMock := mock_spacestorage.NewMockSpaceStorage(ctrl)
	headNotifiableMock := mock_synctree.NewMockHeadNotifiable(ctrl)
	spaceId := "spaceId"
	expectedPayload := objecttree.ObjectTreeCreatePayload{SpaceId: spaceId}
	createObjectTree = func(payload objecttree.ObjectTreeCreatePayload, l list.AclList, create treestorage.TreeStorageCreatorFunc) (objTree objecttree.ObjectTree, err error) {
		require.Equal(t, l, aclListMock)
		require.Equal(t, expectedPayload, payload)
		return objTreeMock, nil
	}

	createSyncClient = syncClientFuncCreator(syncClientMock)
	objTreeMock.EXPECT().Heads().AnyTimes().Return([]string{"h1"})
	headUpdate := &treechangeproto.TreeSyncMessage{}
	headNotifiableMock.EXPECT().UpdateHeads("id", []string{"h1"})
	syncClientMock.EXPECT().CreateHeadUpdate(gomock.Any(), gomock.Nil()).Return(headUpdate)
	syncClientMock.EXPECT().BroadcastAsync(gomock.Eq(headUpdate)).Return(nil)
	objTreeMock.EXPECT().Id().Return("id")
	deps := CreateDeps{
		AclList:        aclListMock,
		SpaceId:        spaceId,
		Payload:        expectedPayload,
		SpaceStorage:   spaceStorageMock,
		SyncStatus:     syncstatus.NewNoOpSyncStatus(),
		HeadNotifiable: headNotifiableMock,
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
	objTreeMock := newTestObjMock(mock_objecttree.NewMockObjectTree(ctrl))
	tr := &syncTree{
		ObjectTree:  objTreeMock,
		SyncHandler: nil,
		syncClient:  syncClientMock,
		listener:    updateListenerMock,
		isClosed:    false,
		syncStatus:  syncstatus.NewNoOpSyncStatus(),
	}

	headUpdate := &treechangeproto.TreeSyncMessage{}
	t.Run("AddRawChanges update", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		payload := objecttree.RawChangesPayload{
			NewHeads:   nil,
			RawChanges: changes,
		}
		expectedRes := objecttree.AddResult{
			Added: changes,
			Mode:  objecttree.Append,
		}
		objTreeMock.EXPECT().AddRawChanges(gomock.Any(), gomock.Eq(payload)).
			Return(expectedRes, nil)
		updateListenerMock.EXPECT().Update(tr)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), gomock.Eq(changes)).Return(headUpdate)
		syncClientMock.EXPECT().BroadcastAsync(gomock.Eq(headUpdate)).Return(nil)
		res, err := tr.AddRawChanges(ctx, payload)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})

	t.Run("AddRawChanges rebuild", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		payload := objecttree.RawChangesPayload{
			NewHeads:   nil,
			RawChanges: changes,
		}

		expectedRes := objecttree.AddResult{
			Added: changes,
			Mode:  objecttree.Rebuild,
		}
		objTreeMock.EXPECT().AddRawChanges(gomock.Any(), gomock.Eq(payload)).
			Return(expectedRes, nil)
		updateListenerMock.EXPECT().Rebuild(tr)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), gomock.Eq(changes)).Return(headUpdate)
		syncClientMock.EXPECT().BroadcastAsync(gomock.Eq(headUpdate)).Return(nil)
		res, err := tr.AddRawChanges(ctx, payload)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})

	t.Run("AddRawChanges nothing", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		payload := objecttree.RawChangesPayload{
			NewHeads:   nil,
			RawChanges: changes,
		}
		expectedRes := objecttree.AddResult{
			Added: changes,
			Mode:  objecttree.Nothing,
		}
		objTreeMock.EXPECT().AddRawChanges(gomock.Any(), gomock.Eq(payload)).
			Return(expectedRes, nil)

		res, err := tr.AddRawChanges(ctx, payload)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})

	t.Run("AddContent", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		content := objecttree.SignableChangeContent{
			Data: []byte("abcde"),
		}
		expectedRes := objecttree.AddResult{
			Mode:  objecttree.Append,
			Added: changes,
		}
		objTreeMock.EXPECT().Id().Return("id").AnyTimes()
		objTreeMock.EXPECT().AddContent(gomock.Any(), gomock.Eq(content)).
			Return(expectedRes, nil)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), gomock.Eq(changes)).Return(headUpdate)
		syncClientMock.EXPECT().BroadcastAsync(gomock.Eq(headUpdate)).Return(nil)
		res, err := tr.AddContent(ctx, content)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})
}
