package settings

import (
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProvider_ProcessChange(t *testing.T) {
	//ctrl := gomock.NewController(t)
	//objTree := mock_tree.NewMockObjectTree(ctrl)
	prov := &provider{}
	//defer ctrl.Finish()

	t.Run("empty model", func(t *testing.T) {
		ch := &objecttree.Change{}
		startId := "startId"
		rootId := "rootId"
		ids := []string{startId}
		otherIds := prov.processChange(ch, rootId, startId, ids)
		require.Equal(t, []string{startId}, otherIds)
	})

	t.Run("changeId is equal to startId", func(t *testing.T) {
		ch := &objecttree.Change{}
		ch.Model = &spacesyncproto.SettingsData{}
		ch.Id = "startId"

		startId := "startId"
		rootId := "rootId"
		ids := []string{startId}
		otherIds := prov.processChange(ch, rootId, startId, ids)
		require.Equal(t, []string{startId}, otherIds)
	})

	t.Run("changeId is equal to rootId, startId is empty", func(t *testing.T) {
		ch := &objecttree.Change{}
		ch.Model = &spacesyncproto.SettingsData{
			Snapshot: &spacesyncproto.SpaceSettingsSnapshot{
				DeletedIds: []string{"id1", "id2"},
			},
		}
		ch.Id = "rootId"

		startId := ""
		rootId := "rootId"
		otherIds := prov.processChange(ch, rootId, startId, nil)
		require.Equal(t, []string{"id1", "id2"}, otherIds)
	})

	t.Run("changeId is equal to rootId, startId is empty", func(t *testing.T) {
		ch := &objecttree.Change{}
		ch.Model = &spacesyncproto.SettingsData{
			Content: []*spacesyncproto.SpaceSettingsContent{
				{&spacesyncproto.SpaceSettingsContent_ObjectDelete{
					ObjectDelete: &spacesyncproto.ObjectDelete{Id: "id1"},
				}},
			},
		}
		ch.Id = "someId"

		startId := "startId"
		rootId := "rootId"
		otherIds := prov.processChange(ch, rootId, startId, nil)
		require.Equal(t, []string{"id1"}, otherIds)
	})
}

func TestProvider_ProvideIds(t *testing.T) {
	ctrl := gomock.NewController(t)
	objTree := mock_objecttree.NewMockObjectTree(ctrl)
	prov := &provider{}
	defer ctrl.Finish()

	t.Run("startId is empty", func(t *testing.T) {
		ch := &objecttree.Change{Id: "rootId"}
		objTree.EXPECT().Root().Return(ch)
		objTree.EXPECT().Id().Return("id")
		objTree.EXPECT().IterateFrom("id", gomock.Any(), gomock.Any()).Return(nil)
		_, _, err := prov.ProvideIds(objTree, "")
		require.NoError(t, err)
	})

	t.Run("startId is not empty", func(t *testing.T) {
		ch := &objecttree.Change{Id: "rootId"}
		objTree.EXPECT().Root().Return(ch)
		objTree.EXPECT().IterateFrom("startId", gomock.Any(), gomock.Any()).Return(nil)
		_, _, err := prov.ProvideIds(objTree, "startId")
		require.NoError(t, err)
	})
}
