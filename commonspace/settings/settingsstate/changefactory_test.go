package settingsstate

import (
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
	"testing"
)

func TestChangeFactory_CreateObjectDeleteChange(t *testing.T) {
	factory := NewChangeFactory()
	state := &State{
		DeletedIds: map[string]struct{}{"1": {}, "2": {}},
		DeleterId:  "del",
	}
	marshalled, err := factory.CreateObjectDeleteChange("3", state, false)
	require.NoError(t, err)
	data := &spacesyncproto.SettingsData{}
	err = proto.Unmarshal(marshalled, data)
	require.NoError(t, err)
	require.Nil(t, data.Snapshot)
	require.Equal(t, "3", data.Content[0].Value.(*spacesyncproto.SpaceSettingsContent_ObjectDelete).ObjectDelete.Id)

	marshalled, err = factory.CreateObjectDeleteChange("3", state, true)
	require.NoError(t, err)
	data = &spacesyncproto.SettingsData{}
	err = proto.Unmarshal(marshalled, data)
	require.NoError(t, err)
	slices.Sort(data.Snapshot.DeletedIds)
	require.Equal(t, &spacesyncproto.SpaceSettingsSnapshot{
		DeletedIds:    []string{"1", "2", "3"},
		DeleterPeerId: "del",
	}, data.Snapshot)
	require.Equal(t, "3", data.Content[0].Value.(*spacesyncproto.SpaceSettingsContent_ObjectDelete).ObjectDelete.Id)
}

func TestChangeFactory_CreateSpaceDeleteChange(t *testing.T) {
	factory := NewChangeFactory()
	state := &State{
		DeletedIds: map[string]struct{}{"1": {}, "2": {}},
	}
	marshalled, err := factory.CreateSpaceDeleteChange("del", state, false)
	require.NoError(t, err)
	data := &spacesyncproto.SettingsData{}
	err = proto.Unmarshal(marshalled, data)
	require.NoError(t, err)
	require.Nil(t, data.Snapshot)
	require.Equal(t, "del", data.Content[0].Value.(*spacesyncproto.SpaceSettingsContent_SpaceDelete).SpaceDelete.DeleterPeerId)

	marshalled, err = factory.CreateSpaceDeleteChange("del", state, true)
	require.NoError(t, err)
	data = &spacesyncproto.SettingsData{}
	err = proto.Unmarshal(marshalled, data)
	require.NoError(t, err)
	slices.Sort(data.Snapshot.DeletedIds)
	require.Equal(t, &spacesyncproto.SpaceSettingsSnapshot{
		DeletedIds:    []string{"1", "2"},
		DeleterPeerId: "del",
	}, data.Snapshot)
	require.Equal(t, "del", data.Content[0].Value.(*spacesyncproto.SpaceSettingsContent_SpaceDelete).SpaceDelete.DeleterPeerId)
}
