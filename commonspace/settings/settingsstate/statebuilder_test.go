package settingsstate

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree/mock_objecttree"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

func TestStateBuilder_ProcessChange(t *testing.T) {
	sb := &stateBuilder{}
	rootId := "rootId"
	deletedId := "deletedId"

	t.Run("empty model", func(t *testing.T) {
		ch := &objecttree.Change{}
		newSt := sb.processChange(ch, rootId, &State{
			DeletedIds: map[string]struct{}{deletedId: struct{}{}},
		})
		require.Equal(t, map[string]struct{}{deletedId: struct{}{}}, newSt.DeletedIds)
	})

	t.Run("correct space deleted", func(t *testing.T) {
		keys, _ := accountdata.NewRandom()
		ch := &objecttree.Change{
			Identity: keys.SignKey.GetPublic(),
		}
		ch.PreviousIds = []string{"someId"}
		ch.Model = &spacesyncproto.SettingsData{
			Content: []*spacesyncproto.SpaceSettingsContent{
				{Value: &spacesyncproto.SpaceSettingsContent_SpaceDelete{
					SpaceDelete: &spacesyncproto.SpaceDelete{DeleterPeerId: "peerId"},
				}},
			},
		}
		ch.Id = "someId"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		newSt := sb.processChange(ch, rootId, NewState())
		fmt.Println(newSt)
	})

	t.Run("changeId is equal to startId, LastIteratedId is equal to startId", func(t *testing.T) {
		ch := &objecttree.Change{}
		ch.Model = &spacesyncproto.SettingsData{
			Content: []*spacesyncproto.SpaceSettingsContent{
				{Value: &spacesyncproto.SpaceSettingsContent_ObjectDelete{
					ObjectDelete: &spacesyncproto.ObjectDelete{Id: deletedId},
				}},
			},
		}
		ch.Id = "startId"
		startId := "startId"
		newSt := sb.processChange(ch, rootId, &State{
			DeletedIds:     map[string]struct{}{deletedId: struct{}{}},
			LastIteratedId: startId,
		})
		require.Equal(t, map[string]struct{}{deletedId: struct{}{}}, newSt.DeletedIds)
	})

	t.Run("changeId is equal to rootId", func(t *testing.T) {
		ch := &objecttree.Change{}
		ch.PreviousIds = []string{"someId"}
		ch.Model = &spacesyncproto.SettingsData{
			Snapshot: &spacesyncproto.SpaceSettingsSnapshot{
				DeletedIds: []string{"id1", "id2"},
			},
		}
		ch.Id = "rootId"
		newSt := sb.processChange(ch, rootId, NewState())
		require.Equal(t, map[string]struct{}{"id1": struct{}{}, "id2": struct{}{}}, newSt.DeletedIds)
	})

	t.Run("changeId is not equal to lastIteratedId or rootId", func(t *testing.T) {
		ch := &objecttree.Change{}
		ch.PreviousIds = []string{"someId"}
		ch.Model = &spacesyncproto.SettingsData{
			Content: []*spacesyncproto.SpaceSettingsContent{
				{Value: &spacesyncproto.SpaceSettingsContent_ObjectDelete{
					ObjectDelete: &spacesyncproto.ObjectDelete{Id: deletedId},
				}},
			},
		}
		ch.Id = "someId"
		newSt := sb.processChange(ch, rootId, NewState())
		require.Equal(t, map[string]struct{}{deletedId: struct{}{}}, newSt.DeletedIds)
	})
}

func TestStateBuilder_Build(t *testing.T) {
	ctrl := gomock.NewController(t)
	objTree := mock_objecttree.NewMockObjectTree(ctrl)
	sb := &stateBuilder{}
	defer ctrl.Finish()

	t.Run("state is nil", func(t *testing.T) {
		ch := &objecttree.Change{Id: "rootId"}
		objTree.EXPECT().Root().Return(ch)
		objTree.EXPECT().IterateFrom("rootId", gomock.Any(), gomock.Any()).Return(nil)
		_, err := sb.Build(objTree, nil)
		require.NoError(t, err)
	})

	t.Run("state is non-empty", func(t *testing.T) {
		ch := &objecttree.Change{Id: "rootId"}
		objTree.EXPECT().Root().Return(ch)
		objTree.EXPECT().IterateFrom("someId", gomock.Any(), gomock.Any()).Return(nil)
		_, err := sb.Build(objTree, &State{LastIteratedId: "someId"})
		require.NoError(t, err)
	})
}
