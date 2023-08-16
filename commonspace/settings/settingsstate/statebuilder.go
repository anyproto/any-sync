package settingsstate

import (
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/gogo/protobuf/proto"
)

type StateBuilder interface {
	Build(tree objecttree.ReadableObjectTree, state *State) (*State, error)
}

func NewStateBuilder() StateBuilder {
	return &stateBuilder{}
}

type stateBuilder struct {
}

func (s *stateBuilder) Build(tr objecttree.ReadableObjectTree, oldState *State) (state *State, err error) {
	var (
		rootId  = tr.Root().Id
		startId = rootId
	)
	state = oldState
	if state == nil {
		state = NewState()
	} else if state.LastIteratedId != "" {
		startId = state.LastIteratedId
	}

	process := func(change *objecttree.Change) bool {
		state = s.processChange(change, rootId, state)
		state.LastIteratedId = change.Id
		return true
	}
	convert := func(decrypted []byte) (res any, err error) {
		deleteChange := &spacesyncproto.SettingsData{}
		err = proto.Unmarshal(decrypted, deleteChange)
		if err != nil {
			return nil, err
		}
		return deleteChange, nil
	}
	err = tr.IterateFrom(startId, convert, process)
	return
}

func (s *stateBuilder) processChange(change *objecttree.Change, rootId string, state *State) *State {
	// ignoring root change which has empty model or startId change
	if len(change.PreviousIds) == 0 || state.LastIteratedId == change.Id {
		return state
	}

	deleteChange := change.Model.(*spacesyncproto.SettingsData)
	// getting data from snapshot if we start from it
	if change.Id == rootId {
		state = NewStateFromSnapshot(deleteChange.Snapshot, rootId)
		return state
	}

	// otherwise getting data from content
	for _, cnt := range deleteChange.Content {
		switch {
		case cnt.GetObjectDelete() != nil:
			state.DeletedIds[cnt.GetObjectDelete().GetId()] = struct{}{}
		}
	}
	return state
}
