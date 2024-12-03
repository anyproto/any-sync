package synctree

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener/mock_updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage/mock_spacestorage"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/syncstatus/mock_syncstatus"
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

type testObjMock struct {
	*mock_objecttree.MockObjectTree
}

func newTestObjMock(obj *mock_objecttree.MockObjectTree) *testObjMock {
	return &testObjMock{MockObjectTree: obj}
}

func (t *testObjMock) Lock() {
}

func (t *testObjMock) Unlock() {
}

func (s syncTreeMatcher) String() string {
	return ""
}

type testTreeGetter struct {
	treeStorage objecttree.Storage
	peerId      string
}

func (t testTreeGetter) getTree(ctx context.Context) (treeStorage objecttree.Storage, peerId string, err error) {
	return t.treeStorage, t.peerId, nil
}

type fixture struct {
	ctrl           *gomock.Controller
	syncClient     *mock_synctree.MockSyncClient
	objTree        *testObjMock
	listener       *mock_updatelistener.MockUpdateListener
	headNotifiable *mock_synctree.MockHeadNotifiable
	syncStatus     *mock_syncstatus.MockStatusUpdater
	spaceStorage   *mock_spacestorage.MockSpaceStorage
	deps           BuildDeps
}

func (fx *fixture) finish() {
	fx.ctrl.Finish()
}

func newFixture(t *testing.T) *fixture {
	ctrl := gomock.NewController(t)
	syncClient := mock_synctree.NewMockSyncClient(ctrl)
	objTree := newTestObjMock(mock_objecttree.NewMockObjectTree(ctrl))
	listener := mock_updatelistener.NewMockUpdateListener(ctrl)
	headNotifiable := mock_synctree.NewMockHeadNotifiable(ctrl)
	syncStatus := mock_syncstatus.NewMockStatusUpdater(ctrl)
	spaceStorage := mock_spacestorage.NewMockSpaceStorage(ctrl)
	deps := BuildDeps{
		SyncClient:     syncClient,
		Listener:       listener,
		HeadNotifiable: headNotifiable,
		BuildObjectTree: func(objecttree.Storage, list.AclList) (objecttree.ObjectTree, error) {
			return objTree, nil
		},
		SyncStatus:   syncStatus,
		SpaceStorage: spaceStorage,
	}
	return &fixture{
		ctrl:           ctrl,
		syncClient:     syncClient,
		objTree:        objTree,
		listener:       listener,
		headNotifiable: headNotifiable,
		syncStatus:     syncStatus,
		spaceStorage:   spaceStorage,
		deps:           deps,
	}
}

var ctx = context.Background()

func Test_BuildSyncTree(t *testing.T) {
	t.Run("build from peer ok", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish()
		newTreeGetter = func(deps BuildDeps, treeId string) treeGetter {
			return testTreeGetter{treeStorage: nil, peerId: "peerId"}
		}
		fx.objTree.EXPECT().Heads().AnyTimes().Return([]string{"headId"})
		fx.objTree.EXPECT().Id().AnyTimes().Return("id")
		fx.objTree.EXPECT().IsDerived().Return(false)
		fx.headNotifiable.EXPECT().UpdateHeads("id", []string{"headId"})
		fx.listener.EXPECT().Rebuild(gomock.Any())
		headUpdate := &objectmessages.HeadUpdate{
			Bytes: []byte("bytes"),
		}
		fx.syncClient.EXPECT().CreateHeadUpdate(gomock.Any(), "", nil).Return(headUpdate, nil)
		fx.syncClient.EXPECT().Broadcast(gomock.Any(), headUpdate)
		fx.syncStatus.EXPECT().ObjectReceive("peerId", "id", []string{"headId"})
		res, err := BuildSyncTreeOrGetRemote(ctx, "id", fx.deps)
		require.NoError(t, err)
		require.NotNil(t, res)
	})
	t.Run("build from storage = empty peer", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish()
		newTreeGetter = func(deps BuildDeps, treeId string) treeGetter {
			return testTreeGetter{treeStorage: nil, peerId: ""}
		}
		fx.objTree.EXPECT().Heads().AnyTimes().Return([]string{"headId"})
		fx.objTree.EXPECT().Id().AnyTimes().Return("id")
		fx.headNotifiable.EXPECT().UpdateHeads("id", []string{"headId"})
		fx.listener.EXPECT().Rebuild(gomock.Any())
		res, err := BuildSyncTreeOrGetRemote(ctx, "id", fx.deps)
		require.NoError(t, err)
		require.NotNil(t, res)
	})
}

func Test_PutSyncTree(t *testing.T) {
	t.Run("put sync tree ok", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish()
		fx.spaceStorage.EXPECT().CreateTreeStorage(gomock.Any()).Return(nil, nil)
		fx.objTree.EXPECT().Heads().AnyTimes().Return([]string{"headId"})
		fx.objTree.EXPECT().Id().AnyTimes().Return("id")
		fx.objTree.EXPECT().IsDerived().Return(false)
		fx.headNotifiable.EXPECT().UpdateHeads("id", []string{"headId"})
		fx.listener.EXPECT().Rebuild(gomock.Any())
		headUpdate := &objectmessages.HeadUpdate{
			Bytes: []byte("bytes"),
		}
		fx.syncClient.EXPECT().CreateHeadUpdate(gomock.Any(), "", nil).Return(headUpdate, nil)
		fx.syncClient.EXPECT().Broadcast(gomock.Any(), headUpdate)
		fx.spaceStorage.EXPECT().TreeDeletedStatus("rootId").Return("", nil)
		res, err := PutSyncTree(ctx, treestorage.TreeStorageCreatePayload{
			RootRawChange: &treechangeproto.RawTreeChangeWithId{
				Id: "rootId",
			},
		}, fx.deps)
		require.NoError(t, err)
		require.NotNil(t, res)
	})
	t.Run("put sync tree already deleted", func(t *testing.T) {
		fx := newFixture(t)
		defer fx.finish()
		fx.spaceStorage.EXPECT().TreeDeletedStatus("rootId").Return("deleted", nil)
		res, err := PutSyncTree(ctx, treestorage.TreeStorageCreatePayload{
			RootRawChange: &treechangeproto.RawTreeChangeWithId{
				Id: "rootId",
			},
		}, fx.deps)
		require.Equal(t, spacestorage.ErrTreeStorageAlreadyDeleted, err)
		require.Nil(t, res)
	})
}

func Test_SyncTree(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	updateListenerMock := mock_updatelistener.NewMockUpdateListener(ctrl)
	syncClientMock := mock_synctree.NewMockSyncClient(ctrl)
	objTreeMock := newTestObjMock(mock_objecttree.NewMockObjectTree(ctrl))
	objTreeMock.EXPECT().Id().AnyTimes().Return("id")
	syncStatusMock := mock_syncstatus.NewMockStatusUpdater(ctrl)
	notifiableMock := mock_synctree.NewMockHeadNotifiable(ctrl)

	tr := &syncTree{
		ObjectTree: objTreeMock,
		syncClient: syncClientMock,
		listener:   updateListenerMock,
		notifiable: notifiableMock,
		isClosed:   false,
		syncStatus: syncStatusMock,
	}

	headUpdate := &objectmessages.HeadUpdate{}
	t.Run("AddRawChangesFromPeer update", func(t *testing.T) {
		changes := []objecttree.StorageChange{{Id: "some"}}
		rawChanges := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		payload := objecttree.RawChangesPayload{
			NewHeads:   []string{"headId1"},
			RawChanges: rawChanges,
		}
		expectedRes := objecttree.AddResult{
			Added:    changes,
			OldHeads: []string{"headId"},
			Heads:    []string{"headId1"},
			Mode:     objecttree.Append,
		}
		objTreeMock.EXPECT().Heads().Return([]string{"headId"}).Times(2)
		objTreeMock.EXPECT().Heads().Return([]string{"headId1"}).Times(1)
		objTreeMock.EXPECT().HasChanges(gomock.Any()).AnyTimes().Return(false)
		objTreeMock.EXPECT().AddRawChangesWithUpdater(gomock.Any(), gomock.Eq(payload), gomock.Any()).
			DoAndReturn(func(ctx context.Context, changes objecttree.RawChangesPayload, updater objecttree.Updater) (addResult objecttree.AddResult, err error) {
				err = updater(objTreeMock, objecttree.Append)
				if err != nil {
					return objecttree.AddResult{}, err
				}
				return expectedRes, nil
			})
		notifiableMock.EXPECT().UpdateHeads("id", []string{"headId1"})
		updateListenerMock.EXPECT().Update(tr).Return(nil)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), "peerId", gomock.Eq(changes)).Return(headUpdate, nil)
		syncClientMock.EXPECT().Broadcast(gomock.Any(), gomock.Eq(headUpdate))
		syncStatusMock.EXPECT().HeadsApply("peerId", "id", []string{"headId1"}, true)
		res, err := tr.AddRawChangesFromPeer(ctx, "peerId", payload)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})
	t.Run("AddRawChangesFromPeer rebuild", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		payload := objecttree.RawChangesPayload{
			NewHeads:   []string{"headId1"},
			RawChanges: changes,
		}
		expectedRes := objecttree.AddResult{
			Added:    changes,
			OldHeads: []string{"headId"},
			Heads:    []string{"headId1"},
			Mode:     objecttree.Rebuild,
		}
		objTreeMock.EXPECT().Heads().Return([]string{"headId"}).Times(2)
		objTreeMock.EXPECT().Heads().Return([]string{"headId1"}).Times(1)
		objTreeMock.EXPECT().HasChanges(gomock.Any()).AnyTimes().Return(false)
		objTreeMock.EXPECT().AddRawChangesWithUpdater(gomock.Any(), gomock.Eq(payload), gomock.Any()).
			DoAndReturn(func(ctx context.Context, changes objecttree.RawChangesPayload, updater objecttree.Updater) (addResult objecttree.AddResult, err error) {
				err = updater(objTreeMock, objecttree.Rebuild)
				if err != nil {
					return objecttree.AddResult{}, err
				}
				return expectedRes, nil
			})
		notifiableMock.EXPECT().UpdateHeads("id", []string{"headId1"})
		updateListenerMock.EXPECT().Rebuild(tr).Return(nil)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), "peerId", gomock.Eq(changes)).Return(headUpdate, nil)
		syncClientMock.EXPECT().Broadcast(gomock.Any(), gomock.Eq(headUpdate))
		syncStatusMock.EXPECT().HeadsApply("peerId", "id", []string{"headId1"}, true)
		res, err := tr.AddRawChangesFromPeer(ctx, "peerId", payload)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})
	t.Run("AddRawChangesFromPeer hasHeads nothing", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		payload := objecttree.RawChangesPayload{
			NewHeads:   []string{"headId"},
			RawChanges: changes,
		}
		expectedRes := objecttree.AddResult{
			OldHeads: []string{"headId"},
			Heads:    []string{"headId"},
			Mode:     objecttree.Nothing,
		}
		objTreeMock.EXPECT().Heads().AnyTimes().Return([]string{"headId"})
		syncStatusMock.EXPECT().HeadsApply("peerId", "id", []string{"headId"}, true)
		objTreeMock.EXPECT().HasChanges(gomock.Any()).AnyTimes().Return(true)
		objTreeMock.EXPECT().Id().Return("id").AnyTimes()
		res, err := tr.AddRawChangesFromPeer(ctx, "peerId", payload)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})
	t.Run("AddRawChangesFromPeer nothing", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		payload := objecttree.RawChangesPayload{
			NewHeads:   []string{"headId2"},
			RawChanges: changes,
		}
		expectedRes := objecttree.AddResult{
			OldHeads: []string{"headId"},
			Heads:    []string{"headId"},
			Mode:     objecttree.Nothing,
		}
		objTreeMock.EXPECT().Heads().Return([]string{"headId"}).AnyTimes()
		objTreeMock.EXPECT().HasChanges(gomock.Any()).AnyTimes().Return(false)
		objTreeMock.EXPECT().AddRawChangesWithUpdater(gomock.Any(), gomock.Eq(payload), gomock.Any()).
			DoAndReturn(func(ctx context.Context, changes objecttree.RawChangesPayload, updater objecttree.Updater) (addResult objecttree.AddResult, err error) {
				err = updater(objTreeMock, objecttree.Nothing)
				if err != nil {
					return objecttree.AddResult{}, err
				}
				return expectedRes, nil
			})
		res, err := tr.AddRawChangesFromPeer(ctx, "peerId", payload)
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
			Heads: []string{"headId"},
			Added: changes,
		}
		objTreeMock.EXPECT().Id().Return("id").AnyTimes()
		objTreeMock.EXPECT().AddContentWithValidator(gomock.Any(), gomock.Eq(content), gomock.Any()).
			Return(expectedRes, nil)
		syncStatusMock.EXPECT().HeadsChange("id", []string{"headId"})
		notifiableMock.EXPECT().UpdateHeads("id", []string{"headId"})
		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), "", gomock.Eq(changes)).Return(headUpdate, nil)
		syncClientMock.EXPECT().Broadcast(gomock.Any(), gomock.Eq(headUpdate))
		res, err := tr.AddContent(ctx, content)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})
}
