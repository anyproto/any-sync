package settingsdocument

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/gogo/protobuf/proto"
)

type DeletedIdsProvider interface {
	ProvideIds(tr tree.ObjectTree, startId string) (ids []string, lastId string, err error)
}

type provider struct{}

func (p *provider) convert(decrypted []byte) (res any, err error) {
	deleteChange := &spacesyncproto.SettingsData{}
	err = proto.Unmarshal(decrypted, deleteChange)
	if err != nil {
		return nil, err
	}
	return deleteChange, nil
}

func (p *provider) processChange(change *tree.Change, tr tree.ObjectTree, startId string, ids []string) []string {
	// ignoring root change which has empty model or startId change
	if change.Model == nil || (change.Id == startId && startId != "") {
		return ids
	}

	deleteChange := change.Model.(*spacesyncproto.SettingsData)
	// getting data from snapshot if we start from it
	if change.Id == tr.Root().Id {
		ids = deleteChange.Snapshot.DeletedIds
		return ids
	}

	// otherwise getting data from content
	for _, cnt := range deleteChange.Content {
		if cnt.GetObjectDelete() != nil {
			ids = append(ids, cnt.GetObjectDelete().GetId())
		}
	}
	return ids
}

func (p *provider) ProvideIds(tr tree.ObjectTree, startId string) (ids []string, lastId string, err error) {
	process := func(change *tree.Change) bool {
		lastId = change.Id
		ids = p.processChange(change, tr, startId, ids)
		return true
	}
	if startId == "" {
		err = tr.IterateFrom(tr.ID(), p.convert, process)
	} else {
		err = tr.IterateFrom(startId, p.convert, process)
	}
	return
}
