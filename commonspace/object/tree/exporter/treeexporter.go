package exporter

import (
	"github.com/anytypeio/any-sync/commonspace/object/acl/liststorage"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treestorage"
)

type DataConverter interface {
	Unmarshall(decrypted []byte) (any, error)
	Convert(model any) (any, error)
}

type TreeExporterParams struct {
	ListStorageExporter liststorage.Exporter
	TreeStorageExporter treestorage.Exporter
	DataConverter       DataConverter
}

type TreeExporter interface {
	ExportUnencrypted(tree objecttree.ReadableObjectTree) (err error)
}

func NewTreeExporter(params TreeExporterParams) TreeExporter {
	return nil
}
