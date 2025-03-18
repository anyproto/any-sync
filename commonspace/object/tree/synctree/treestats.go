package synctree

import (
	"sync"
)

type TreeStatsCollector struct {
	trees   map[string]*syncTree
	mutex   sync.Mutex
	spaceId string
}

func NewTreeStatsCollector(spaceId string) *TreeStatsCollector {
	return &TreeStatsCollector{
		trees:   make(map[string]*syncTree),
		spaceId: spaceId,
	}
}

func (t *TreeStatsCollector) Register(tree *syncTree) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.trees[tree.Id()] = tree
}

func (t *TreeStatsCollector) Collect() []TreeStats {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	stats := make([]TreeStats, 0, len(t.trees))
	for _, tree := range t.trees {
		tree.Lock()
		stats = append(stats, TreeStats{
			TreeLen:         tree.Len(),
			SnapshotCounter: tree.Root().SnapshotCounter,
			Heads:           tree.Heads(),
			ObjectId:        tree.Id(),
			SpaceId:         t.spaceId,
			BuildTimeMillis: int(tree.buildTime.Milliseconds()),
		})
		tree.Unlock()
	}
	return stats
}

func (t *TreeStatsCollector) Unregister(tree SyncTree) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	delete(t.trees, tree.Id())
}

type TreeStats struct {
	TreeLen         int      `json:"tree_len"`
	SnapshotCounter int      `json:"snapshot_counter"`
	Heads           []string `json:"heads"`
	ObjectId        string   `json:"object_id"`
	SpaceId         string   `json:"space_id"`
	BuildTimeMillis int      `json:"build_time_millis"`
}
