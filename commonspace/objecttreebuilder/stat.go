package objecttreebuilder

import "github.com/anyproto/any-sync/commonspace/object/tree/synctree"

type debugStat struct {
	TreeStats []synctree.TreeStats `json:"tree_stats"`
	SpaceId   string               `json:"space_id"`
}
