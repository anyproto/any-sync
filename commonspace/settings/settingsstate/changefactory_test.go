package settingsstate

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

func TestChangeFactory_CreateObjectDeleteChange(t *testing.T) {
	factory := NewChangeFactory()
	state := &State{
		DeletedIds: map[string]struct{}{"1": {}, "2": {}},
	}
	marshalled, err := factory.CreateObjectDeleteChange("3", state, false)
	require.NoError(t, err)
	data := &spacesyncproto.SettingsData{}
	err = data.UnmarshalVT(marshalled)
	require.NoError(t, err)
	require.Nil(t, data.Snapshot)
	require.Equal(t, "3", data.Content[0].Value.(*spacesyncproto.SpaceSettingsContent_ObjectDelete).ObjectDelete.Id)

	marshalled, err = factory.CreateObjectDeleteChange("3", state, true)
	require.NoError(t, err)
	data = &spacesyncproto.SettingsData{}
	err = data.UnmarshalVT(marshalled)
	require.NoError(t, err)
	slices.Sort(data.Snapshot.DeletedIds)
	require.Equal(t, &spacesyncproto.SpaceSettingsSnapshot{
		DeletedIds: []string{"1", "2", "3"},
	}, data.Snapshot)
	require.Equal(t, "3", data.Content[0].Value.(*spacesyncproto.SpaceSettingsContent_ObjectDelete).ObjectDelete.Id)
}
