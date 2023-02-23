package settingsstate

import (
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/gogo/protobuf/proto"
)

type StateBuilder interface {
	Build(tree objecttree.ObjectTree, state *State, isUpdate bool) (*State, error)
}

func NewStateBuilder() StateBuilder {
	return &stateBuilder{}
}

type stateBuilder struct {
}

func (s *stateBuilder) Build(tr objecttree.ObjectTree, oldState *State, isUpdate bool) (state *State, err error) {
	state = oldState

	if !isUpdate || state == nil {
		state = &State{}
	}
	var (
		rootId  = tr.Root().Id
		startId = state.LastIteratedId
	)
	process := func(change *objecttree.Change) bool {
		state.LastIteratedId = change.Id
		state = s.processChange(change, rootId, startId, state)
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

	if startId == "" {
		startId = rootId
	}
	err = tr.IterateFrom(state.LastIteratedId, convert, process)
	return
}

func (s *stateBuilder) processChange(change *objecttree.Change, rootId, startId string, state *State) *State {
	// ignoring root change which has empty model or startId change
	if change.Model == nil || (change.Id == startId && startId != "") {
		return state
	}

	deleteChange := change.Model.(*spacesyncproto.SettingsData)
	// getting data from snapshot if we start from it
	if change.Id == rootId {
		state = &State{
			DeletedIds:     deleteChange.Snapshot.DeletedIds,
			DeleterId:      deleteChange.Snapshot.DeleterPeerId,
			LastIteratedId: rootId,
		}
		return state
	}

	// otherwise getting data from content
	for _, cnt := range deleteChange.Content {
		switch {
		case cnt.GetObjectDelete() != nil:
			state.DeletedIds = append(state.DeletedIds, cnt.GetObjectDelete().GetId())
		case cnt.GetSpaceDelete() != nil:
			state.DeleterId = cnt.GetSpaceDelete().GetDeleterPeerId()
		}
	}
	return state
}
