package exporter

import (
	"github.com/anyproto/any-sync/commonspace/object/acl/liststorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/util/crypto"
)

type DataConverter interface {
	Unmarshall(decrypted []byte) (any, error)
	Marshall(model any) ([]byte, error)
}

type TreeExporterParams struct {
	ListStorageExporter liststorage.Exporter
	TreeStorageExporter treestorage.Exporter
	DataConverter       DataConverter
}

type TreeExporter interface {
	ExportUnencrypted(tree objecttree.ReadableObjectTree) (err error)
}

type treeExporter struct {
	listExporter liststorage.Exporter
	treeExporter treestorage.Exporter
	converter    DataConverter
}

func NewTreeExporter(params TreeExporterParams) TreeExporter {
	return &treeExporter{
		listExporter: params.ListStorageExporter,
		treeExporter: params.TreeStorageExporter,
		converter:    params.DataConverter,
	}
}

func (t *treeExporter) ExportUnencrypted(tree objecttree.ReadableObjectTree) (err error) {
	lst := tree.AclList()
	// this exports root which should be enough before we implement acls
	_, err = t.listExporter.ListStorage(lst.Root())
	if err != nil {
		return
	}
	treeStorage, err := t.treeExporter.TreeStorage(tree.Header())
	if err != nil {
		return
	}
	changeBuilder := objecttree.NewChangeBuilder(crypto.NewKeyStorage(), tree.Header())
	putStorage := func(change *objecttree.Change) (err error) {
		var raw *treechangeproto.RawTreeChangeWithId
		raw, err = changeBuilder.Marshall(change)
		if err != nil {
			return
		}
		return treeStorage.AddRawChange(raw)
	}
	err = tree.IterateRoot(t.converter.Unmarshall, func(change *objecttree.Change) bool {
		if change.Id == tree.Id() {
			err = putStorage(change)
			return err == nil
		}
		var data []byte
		data, err = t.converter.Marshall(change.Model)
		if err != nil {
			return false
		}
		// that means that change is unencrypted
		change.ReadKeyId = ""
		change.Data = data
		err = putStorage(change)
		return err == nil
	})
	if err != nil {
		return
	}
	return treeStorage.SetHeads(tree.Heads())
}
