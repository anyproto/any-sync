package settings

import (
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/gogo/protobuf/proto"
	"time"
)

type StateBuilder interface {
	Build(tree objecttree.ObjectTree, isUpdate bool) (*State, error)
}

func newStateBuilder() StateBuilder {
	return &stateBuilder{}
}

type stateBuilder struct {
	state *State
}

func (s *stateBuilder) Build(tr objecttree.ObjectTree, isUpdate bool) (state *State, err error) {
	state = s.state
	defer func() {
		if err == nil {
			s.state = state
		}
	}()

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
			DeletedIds:        deleteChange.Snapshot.DeletedIds,
			SpaceDeletionDate: time.Unix(0, deleteChange.Snapshot.SpaceDeletionTimestamp),
			LastIteratedId:    rootId,
		}
		return state
	}

	// otherwise getting data from content
	for _, cnt := range deleteChange.Content {
		switch {
		case cnt.GetObjectDelete() != nil:
			state.DeletedIds = append(state.DeletedIds, cnt.GetObjectDelete().GetId())
		case cnt.GetSpaceDelete() != nil:
			state.SpaceDeletionDate = time.Unix(0, cnt.GetSpaceDelete().GetTimestamp())
		}
	}
	return state
}
