package synctree

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/mock_synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener/mock_updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/objectsync"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/nodeconf"
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

func syncClientFuncCreator(client SyncClient) func(spaceId string, factory RequestFactory, objectSync objectsync.ObjectSync, configuration nodeconf.NodeConf) SyncClient {
	return func(spaceId string, factory RequestFactory, objectSync objectsync.ObjectSync, configuration nodeconf.NodeConf) SyncClient {
		return client
	}
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
	t.Run("AddRawChangesFromPeer update", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		payload := objecttree.RawChangesPayload{
			NewHeads:   nil,
			RawChanges: changes,
		}
		expectedRes := objecttree.AddResult{
			Added: changes,
			Mode:  objecttree.Append,
		}
		objTreeMock.EXPECT().Heads().AnyTimes().Return([]string{"headId"})
		objTreeMock.EXPECT().HasChanges(gomock.Any()).AnyTimes().Return(false)
		objTreeMock.EXPECT().AddRawChanges(gomock.Any(), gomock.Eq(payload)).
			Return(expectedRes, nil)
		updateListenerMock.EXPECT().Update(tr)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), gomock.Eq(changes)).Return(headUpdate)
		syncClientMock.EXPECT().Broadcast(gomock.Eq(headUpdate))
		res, err := tr.AddRawChangesFromPeer(ctx, "peerId", payload)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})

	t.Run("AddRawChangesFromPeer rebuild", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		payload := objecttree.RawChangesPayload{
			NewHeads:   nil,
			RawChanges: changes,
		}

		expectedRes := objecttree.AddResult{
			Added: changes,
			Mode:  objecttree.Rebuild,
		}
		objTreeMock.EXPECT().Heads().AnyTimes().Return([]string{"headId"})
		objTreeMock.EXPECT().AddRawChanges(gomock.Any(), gomock.Eq(payload)).
			Return(expectedRes, nil)
		updateListenerMock.EXPECT().Rebuild(tr)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), gomock.Eq(changes)).Return(headUpdate)
		syncClientMock.EXPECT().Broadcast(gomock.Eq(headUpdate))
		res, err := tr.AddRawChangesFromPeer(ctx, "peerId", payload)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})

	t.Run("AddRawChangesFromPeer nothing", func(t *testing.T) {
		changes := []*treechangeproto.RawTreeChangeWithId{{Id: "some"}}
		payload := objecttree.RawChangesPayload{
			NewHeads:   nil,
			RawChanges: changes,
		}
		expectedRes := objecttree.AddResult{
			Added: changes,
			Mode:  objecttree.Nothing,
		}
		objTreeMock.EXPECT().Heads().AnyTimes().Return([]string{"headId"})
		objTreeMock.EXPECT().AddRawChanges(gomock.Any(), gomock.Eq(payload)).
			Return(expectedRes, nil)

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
			Added: changes,
		}
		objTreeMock.EXPECT().Id().Return("id").AnyTimes()
		objTreeMock.EXPECT().AddContent(gomock.Any(), gomock.Eq(content)).
			Return(expectedRes, nil)

		syncClientMock.EXPECT().CreateHeadUpdate(gomock.Eq(tr), gomock.Eq(changes)).Return(headUpdate)
		syncClientMock.EXPECT().Broadcast(gomock.Eq(headUpdate))
		res, err := tr.AddContent(ctx, content)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})
}
