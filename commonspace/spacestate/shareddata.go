package spacestate

import (
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"sync/atomic"
)

const CName = "common.commonspace.shareddata"

type SpaceActions interface {
	OnObjectDelete(id string)
	OnSpaceDelete()
}

type SpaceState struct {
	SpaceId         string
	SpaceIsDeleted  *atomic.Bool
	SpaceIsClosed   *atomic.Bool
	TreesUsed       *atomic.Int32
	AclList         list.AclList
	SpaceStorage    spacestorage.SpaceStorage
	TreeBuilderFunc objecttree.BuildObjectTreeFunc
	Actions         SpaceActions
}

func (s *SpaceState) Init(a *app.App) (err error) {
	return nil
}

func (s *SpaceState) Name() (name string) {
	return CName
}
