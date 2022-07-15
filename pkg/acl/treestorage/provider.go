package treestorage

import "errors"

var UnknownTreeId = errors.New("tree does not exist")

type Provider interface {
	TreeStorage(treeId string) (TreeStorage, error)
	InsertTree(tree TreeStorage) error
}
