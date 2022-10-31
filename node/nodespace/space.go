package nodespace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
)

func newNodeSpace(cc commonspace.Space) (commonspace.Space, error) {
	return &nodeSpace{cc}, nil
}

type nodeSpace struct {
	commonspace.Space
}

func (s *nodeSpace) Init(ctx context.Context) (err error) {
	// try to push acl to consensus node
	//
	return s.Space.Init(ctx)
}

func (s *nodeSpace) Close() (err error) {
	return s.Space.Close()
}
