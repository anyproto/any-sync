package spacestate

import (
	"sync/atomic"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
)

const CName = "common.commonspace.spacestate"

type SpaceState struct {
	SpaceId         string
	SpaceIsClosed   *atomic.Bool
	TreesUsed       *atomic.Int32
	TreeBuilderFunc objecttree.BuildObjectTreeFunc
}

func (s *SpaceState) Init(a *app.App) (err error) {
	return nil
}

func (s *SpaceState) Name() (name string) {
	return CName
}
