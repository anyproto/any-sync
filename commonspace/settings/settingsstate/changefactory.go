package settingsstate

import (
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
)

type ChangeFactory interface {
	CreateObjectDeleteChange(id string, state *State, isSnapshot bool) (res []byte, err error)
	CreateSpaceDeleteChange(peerId string, state *State, isSnapshot bool) (res []byte, err error)
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
		Snapshot: nil,
	}
	// TODO: add snapshot logic
	res, err = change.Marshal()
	return
}

func (c *changeFactory) CreateSpaceDeleteChange(peerId string, state *State, isSnapshot bool) (res []byte, err error) {
	content := &spacesyncproto.SpaceSettingsContent_SpaceDelete{
		SpaceDelete: &spacesyncproto.SpaceDelete{DeleterPeerId: peerId},
	}
	change := &spacesyncproto.SettingsData{
		Content: []*spacesyncproto.SpaceSettingsContent{
			{Value: content},
		},
		Snapshot: nil,
	}
	// TODO: add snapshot logic
	res, err = change.Marshal()
	return
}
