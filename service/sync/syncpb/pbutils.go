package syncpb

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/acltree"
)

func NewHeadsUpdate(treeId string, headsWithPath []acltree.HeadWithPathToRoot, changes []*aclpb.RawChange) *SyncHeadUpdate {
	var heads []*SyncHead
	for _, headWithPath := range headsWithPath {
		syncHead := &SyncHead{
			Id:           headWithPath.Id,
			SnapshotPath: headWithPath.Path,
		}
		heads = append(heads, syncHead)
	}
	return &SyncHeadUpdate{
		Heads:   heads,
		Changes: changes,
		TreeId:  treeId,
	}
}
