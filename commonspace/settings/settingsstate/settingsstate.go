//go:generate mockgen -destination mock_settingsstate/mock_settingsstate.go github.com/anyproto/any-sync/commonspace/settings/settingsstate StateBuilder,ChangeFactory
package settingsstate

import "github.com/anyproto/any-sync/commonspace/spacesyncproto"

type State struct {
	DeletedIds     map[string]struct{}
	LastIteratedId string
}

func NewState() *State {
	return &State{DeletedIds: map[string]struct{}{}}
}

func NewStateFromSnapshot(snapshot *spacesyncproto.SpaceSettingsSnapshot, lastIteratedId string) *State {
	st := NewState()
	for _, id := range snapshot.DeletedIds {
		st.DeletedIds[id] = struct{}{}
	}
	st.LastIteratedId = lastIteratedId
	return st
}

func (s *State) Exists(id string) bool {
	_, exists := s.DeletedIds[id]
	return exists
}
