//go:generate mockgen -destination mock_treestorage/mock_treestorage.go github.com/anyproto/any-sync/commonspace/object/tree/treestorage TreeStorage
package treestorage

import (
	"errors"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
)

var (
	ErrUnknownTreeId = errors.New("tree does not exist")
)

type TreeStorageCreatePayload struct {
	RootRawChange *treechangeproto.RawTreeChangeWithId
	Changes       []*treechangeproto.RawTreeChangeWithId
	Heads         []string
}
