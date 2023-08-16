package settingsstate

import (
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type ChangeFactory interface {
	CreateObjectDeleteChange(id string, state *State, isSnapshot bool) (res []byte, err error)
}

func NewChangeFactory() ChangeFactory {
	return &changeFactory{}
}

type changeFactory struct {
}

func (c *changeFactory) CreateObjectDeleteChange(id string, state *State, isSnapshot bool) (res []byte, err error) {
	content := &spacesyncproto.SpaceSettingsContent_ObjectDelete{
		ObjectDelete: &spacesyncproto.ObjectDelete{Id: id},
	}
	change := &spacesyncproto.SettingsData{
		Content: []*spacesyncproto.SpaceSettingsContent{
			{Value: content},
		},
	}
	if isSnapshot {
		change.Snapshot = c.makeSnapshot(state, id)
	}
	res, err = change.Marshal()
	return
}

func (c *changeFactory) makeSnapshot(state *State, objectId string) *spacesyncproto.SpaceSettingsSnapshot {
	var (
		deletedIds = make([]string, 0, len(state.DeletedIds)+1)
	)
	if objectId != "" {
		deletedIds = append(deletedIds, objectId)
	}
	for id := range state.DeletedIds {
		deletedIds = append(deletedIds, id)
	}
	return &spacesyncproto.SpaceSettingsSnapshot{
		DeletedIds: deletedIds,
	}
}
