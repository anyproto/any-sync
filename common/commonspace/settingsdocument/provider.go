package settingsdocument

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/gogo/protobuf/proto"
)

type deletedIdsProvider interface {
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

func (p *provider) ProvideIds(tr tree.ObjectTree, startId string) (ids []string, lastId string, err error) {
	processChange := func(change *tree.Change) bool {
		// ignoring first change if startId is not ""
		if change.Id == startId {
			return true
		}
		deleteChange := change.Model.(*spacesyncproto.SettingsData)
		for _, cnt := range deleteChange.Content {
			if cnt.GetObjectDelete() != nil {
				ids = append(ids, cnt.GetObjectDelete().GetId())
			}
		}
		lastId = change.Id
		return true
	}
	if startId == "" {
		err = tr.IterateFrom(tr.ID(), p.convert, processChange)
	} else {
		err = tr.IterateFrom(startId, p.convert, processChange)
	}
	return
}
