package settingsstate

import (
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/gogo/protobuf/proto"
)

type StateBuilder interface {
	Build(tree objecttree.ObjectTree, state *State) (*State, error)
}

func NewStateBuilder() StateBuilder {
	return &stateBuilder{}
}

type stateBuilder struct {
}

func (s *stateBuilder) Build(tr objecttree.ObjectTree, oldState *State) (state *State, err error) {
	var (
		rootId  = tr.Root().Id
		startId = rootId
	)
	state = oldState
	if state == nil {
		state = &State{}
	} else if state.LastIteratedId != "" {
		startId = state.LastIteratedId
	}

	process := func(change *objecttree.Change) bool {
		state = s.processChange(change, rootId, startId, state)
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

func (s *stateBuilder) processChange(change *objecttree.Change, rootId, startId string, state *State) *State {
	// ignoring root change which has empty model or startId change
	if change.Model == nil || state.LastIteratedId == startId {
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
		fmt.Println(cnt.GetSpaceDelete() != nil)
		switch {
		case cnt.GetObjectDelete() != nil:
			state.DeletedIds = append(state.DeletedIds, cnt.GetObjectDelete().GetId())
		case cnt.GetSpaceDelete() != nil:
			fmt.Println(cnt.GetSpaceDelete().GetDeleterPeerId())
			state.DeleterId = cnt.GetSpaceDelete().GetDeleterPeerId()
		}
	}
	return state
}
