package synctree

import (
	"context"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener/mock_updatelistener"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/objectsync"
	"github.com/anyproto/any-sync/commonspace/objectsync/mock_objectsync"
	"github.com/anyproto/any-sync/commonspace/objectsync/syncclient"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

type syncTreeMatcher struct {
	objTree  objecttree.ObjectTree
	client   syncclient.SyncClient
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

func syncClientFuncCreator(client syncclient.SyncClient) func(spaceId string, factory syncclient.RequestFactory, objectSync objectsync.ObjectSync, configuration nodeconf.NodeConf) syncclient.SyncClient {
	return func(spaceId string, factory syncclient.RequestFactory, objectSync objectsync.ObjectSync, configuration nodeconf.NodeConf) syncclient.SyncClient {
		return client
	}
}

func Test_BuildSyncTree(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	updateListenerMock := mock_updatelistener.NewMockUpdateListener(ctrl)
	syncClientMock := mock_objectsync.NewMockSyncClient(ctrl)
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
		syncClientMock.EXPECT().Broadcast(gomock.Any(), gomock.Eq(headUpdate))
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
		syncClientMock.EXPECT().Broadcast(gomock.Any(), gomock.Eq(headUpdate))
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
		syncClientMock.EXPECT().Broadcast(gomock.Any(), gomock.Eq(headUpdate))
		res, err := tr.AddContent(ctx, content)
		require.NoError(t, err)
		require.Equal(t, expectedRes, res)
	})
}
