package synctree

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/response"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
)

func TestFullResponseCollector_CollectResponse(t *testing.T) {
	t.Run("no object tree", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		objTree := mock_objecttree.NewMockObjectTree(ctrl)
		defer ctrl.Finish()
		testPayload := treestorage.TreeStorageCreatePayload{
			RootRawChange: &treechangeproto.RawTreeChangeWithId{Id: "root"},
			Changes:       []*treechangeproto.RawTreeChangeWithId{{Id: "change"}},
			Heads:         []string{"head"},
		}
		resp := &response.Response{
			SpaceId:  "spaceId",
			ObjectId: "objectId",
			Heads:    testPayload.Heads,
			Changes:  testPayload.Changes,
			Root:     testPayload.RootRawChange,
		}
		var validator objecttree.ValidatorFunc = func(payload treestorage.TreeStorageCreatePayload, storageCreator objecttree.TreeStorageCreator, aclList list.AclList) (ret objecttree.ObjectTree, err error) {
			require.Equal(t, testPayload, payload)
			return objTree, nil
		}
		coll := newFullResponseCollector(BuildDeps{
			ValidateObjectTree: validator,
		})
		err := coll.CollectResponse(nil, "peerId", "objectId", resp)
		require.NoError(t, err)
		require.Equal(t, objTree, coll.objectTree)
	})
	t.Run("object tree exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		objTree := mock_objecttree.NewMockObjectTree(ctrl)
		defer ctrl.Finish()
		testPayload := treestorage.TreeStorageCreatePayload{
			RootRawChange: &treechangeproto.RawTreeChangeWithId{Id: "root"},
			Changes:       []*treechangeproto.RawTreeChangeWithId{{Id: "change"}},
			Heads:         []string{"head"},
		}
		resp := &response.Response{
			SpaceId:  "spaceId",
			ObjectId: "objectId",
			Heads:    testPayload.Heads,
			Changes:  testPayload.Changes,
			Root:     testPayload.RootRawChange,
		}
		coll := newFullResponseCollector(BuildDeps{})
		coll.objectTree = objTree
		objTree.EXPECT().AddRawChanges(nil, objecttree.RawChangesPayload{
			NewHeads:   testPayload.Heads,
			RawChanges: testPayload.Changes,
		}).Return(objecttree.AddResult{}, nil)
		err := coll.CollectResponse(nil, "peerId", "objectId", resp)
		require.NoError(t, err)
	})
}
