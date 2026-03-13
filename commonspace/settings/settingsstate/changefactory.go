package settingsstate

import (
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type ChangeFactory interface {
	CreateObjectDeleteChange(ids []string, state *State, isSnapshot bool) (res []byte, err error)
}

func NewChangeFactory() ChangeFactory {
	return &changeFactory{}
}

type changeFactory struct {
}

func (c *changeFactory) CreateObjectDeleteChange(ids []string, state *State, isSnapshot bool) (res []byte, err error) {
	content := make([]*spacesyncproto.SpaceSettingsContent, 0, len(ids))
	for _, id := range ids {
		content = append(content, &spacesyncproto.SpaceSettingsContent{
			Value: &spacesyncproto.SpaceSettingsContent_ObjectDelete{
				ObjectDelete: &spacesyncproto.ObjectDelete{Id: id},
			},
		})
	}
	change := &spacesyncproto.SettingsData{
		Content: content,
	}
	if isSnapshot {
		change.Snapshot = c.makeSnapshot(state, ids...)
	}
	res, err = change.MarshalVT()
	return
}

func (c *changeFactory) makeSnapshot(state *State, objectIds ...string) *spacesyncproto.SpaceSettingsSnapshot {
	var (
		deletedIds = make([]string, 0, len(state.DeletedIds)+len(objectIds))
	)
	deletedIds = append(deletedIds, objectIds...)
	for id := range state.DeletedIds {
		deletedIds = append(deletedIds, id)
	}
	return &spacesyncproto.SpaceSettingsSnapshot{
		DeletedIds: deletedIds,
	}
}
