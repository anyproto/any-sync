package objecttree

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"golang.org/x/tools/container/intsets"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/slice"
)

var (
	log      = logger.NewNamedSugared("common.commonspace.objecttree")
	ErrEmpty = errors.New("database is empty")
)

type treeBuilder struct {
	builder ChangeBuilder
	storage Storage
	ctx     context.Context

	// buffers
	idStack    []string
	loadBuffer []*Change
}

type treeBuilderOpts struct {
	full              bool
	useHeadsSnapshot  bool
	theirHeads        []string
	ourHeads          []string
	ourSnapshotPath   []string
	theirSnapshotPath []string
	newChanges        []*Change
}

func newTreeBuilder(storage Storage, builder ChangeBuilder) *treeBuilder {
	return &treeBuilder{
		storage: storage,
		builder: builder,
	}
}

func (tb *treeBuilder) BuildFull() (*Tree, error) {
	return tb.build(treeBuilderOpts{full: true})
}

var (
	totalSnapshots atomic.Int32
	totalCommon    atomic.Int32
	totalLowest    atomic.Int32
)

func (tb *treeBuilder) buildWithAdded(opts treeBuilderOpts) (*Tree, []*Change, error) {
	cache := make(map[string]*Change)
	tb.ctx = context.Background()
	for _, ch := range opts.newChanges {
		cache[ch.Id] = ch
	}
	var (
		snapshot string
		order    string
	)
	if opts.useHeadsSnapshot {
		maxOrder, lowest, err := tb.lowestSnapshots(nil, opts.ourHeads, "")
		if err != nil {
			return nil, nil, err
		}
		if len(lowest) != 1 {
			snapshot, err = tb.commonSnapshot(lowest)
			if err != nil {
				return nil, nil, err
			}
		} else {
			snapshot = lowest[0]
		}
		order = maxOrder
	} else if !opts.full {
		if len(opts.theirSnapshotPath) == 0 {
			// this is actually not obvious why we should call this here
			// but the idea is if we have no snapshot path, then this can be only in cases
			// when we want to use a common snapshot, otherwise we would provide this path
			// because we always have a snapshot path
			if len(opts.ourSnapshotPath) == 0 {
				common, err := tb.storage.CommonSnapshot(tb.ctx)
				if err != nil {
					return nil, nil, err
				}
				snapshot = common
			} else {
				our := opts.ourSnapshotPath[0]
				_, lowest, err := tb.lowestSnapshots(cache, opts.theirHeads, our)
				if err != nil {
					return nil, nil, err
				}
				if len(lowest) != 1 {
					snapshot, err = tb.commonSnapshot(lowest)
					if err != nil {
						return nil, nil, err
					}
				} else {
					snapshot = lowest[0]
				}
			}
		} else {
			var err error
			snapshot, err = commonSnapshotForTwoPaths(opts.ourSnapshotPath, opts.theirSnapshotPath)
			if err != nil {
				return nil, nil, err
			}
		}
	} else {
		snapshot = tb.storage.Id()
	}
	totalSnapshots.Store(totalSnapshots.Load() + 1)
	snapshotCh, err := tb.storage.Get(tb.ctx, snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get common snapshot %s: %w", snapshot, err)
	}
	rawChange := &treechangeproto.RawTreeChangeWithId{}
	var (
		changes    = make([]*Change, 0, 10)
		newChanges = make([]*Change, 0, 10)
	)
	err = tb.storage.GetAfterOrder(tb.ctx, snapshotCh.OrderId, func(ctx context.Context, storageChange StorageChange) (shouldContinue bool, err error) {
		if order != "" && storageChange.OrderId > order {
			return false, nil
		}
		rawChange.Id = storageChange.Id
		rawChange.RawChange = storageChange.RawChange
		ch, err := tb.builder.Unmarshall(rawChange, false)
		if err != nil {
			return false, err
		}
		ch.OrderId = storageChange.OrderId
		ch.SnapshotCounter = storageChange.SnapshotCounter
		changes = append(changes, ch)
		if _, contains := cache[ch.Id]; contains {
			delete(cache, ch.Id)
		}
		return true, nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get changes after order: %w", err)
	}
	// getting the filtered new changes, we know that we don't have them in storage
	for _, change := range cache {
		newChanges = append(newChanges, change)
	}
	if len(changes) == 0 {
		return nil, nil, ErrEmpty
	}
	tr := &Tree{}
	changes = append(changes, newChanges...)
	added := tr.AddFast(changes...)
	if opts.useHeadsSnapshot {
		// this is important for history, because by default we get everything after the snapshot
		tr.LeaveOnlyBefore(opts.ourHeads)
	}
	if len(newChanges) > 0 {
		newChanges = newChanges[:0]
		for _, change := range added {
			// only those that are both added and are new we deem newChanges
			if _, contains := cache[change.Id]; contains {
				newChanges = append(newChanges, change)
			}
		}
		return tr, newChanges, nil
	}
	return tr, nil, nil
}

func (tb *treeBuilder) build(opts treeBuilderOpts) (tr *Tree, err error) {
	tr, _, err = tb.buildWithAdded(opts)
	return
}

func (tb *treeBuilder) lowestSnapshots(cache map[string]*Change, heads []string, ourSnapshot string) (maxOrder string, snapshots []string, err error) {
	var next, current []string
	if cache != nil {
		for _, ch := range heads {
			head, ok := cache[ch]
			if !ok {
				continue
			}
			if head.SnapshotId != "" {
				next = append(next, head.SnapshotId)
			} else if len(head.PreviousIds) == 0 && head.Id == tb.storage.Id() { // this is a root change
				next = append(next, head.Id)
			} else {
				return "", nil, fmt.Errorf("head with empty snapshot id: %s", head.Id)
			}
		}
	} else {
		for _, head := range heads {
			totalLowest.Store(totalLowest.Load() + 1)
			ch, err := tb.storage.Get(tb.ctx, head)
			if err != nil {
				return "", nil, err
			}
			if ch.OrderId > maxOrder {
				maxOrder = ch.OrderId
			}
			if ch.SnapshotId != "" {
				snapshots = append(snapshots, ch.SnapshotId)
			} else if len(ch.PrevIds) == 0 && ch.Id == tb.storage.Id() { // this is a root change
				snapshots = append(snapshots, ch.Id)
			} else {
				return "", nil, fmt.Errorf("head with empty snapshot id: %s", ch.Id)
			}
		}
	}
	slices.Sort(next)
	next = slice.DiscardDuplicatesSorted(next)
	current = make([]string, 0, len(next))
	var visited []*Change
	for len(next) > 0 {
		current = current[:0]
		current = append(current, next...)
		next = next[:0]
		for _, id := range current {
			if ch, ok := cache[id]; ok {
				if ch.visited {
					continue
				}
				ch.visited = true
				visited = append(visited, ch)
				next = append(next, ch.SnapshotId)
			} else {
				// this is the lowest snapshot from the ones provided
				snapshots = append(snapshots, id)
			}
		}
	}
	for _, ch := range visited {
		ch.visited = false
	}
	if ourSnapshot != "" {
		snapshots = append(snapshots, ourSnapshot)
	}
	slices.Sort(snapshots)
	snapshots = slice.DiscardDuplicatesSorted(snapshots)
	return maxOrder, snapshots, nil
}

func (tb *treeBuilder) commonSnapshot(snapshots []string) (snapshot string, err error) {
	var (
		current       []StorageChange
		lowestCounter = intsets.MaxInt
	)
	// TODO: we should actually check for all changes if they have valid snapshots
	// getting actual snapshots
	for _, id := range snapshots {
		totalCommon.Store(totalCommon.Load() + 1)
		ch, err := tb.storage.Get(tb.ctx, id)
		if err != nil {
			log.Error("failed to get snapshot", zap.String("id", id), zap.Error(err))
			continue
		}
		current = append(current, ch)
		if ch.SnapshotCounter < lowestCounter {
			lowestCounter = ch.SnapshotCounter
		}
	}
	// equalizing counters for each snapshot branch
	for i, ch := range current {
		for ch.SnapshotCounter > lowestCounter {
			totalCommon.Store(totalCommon.Load() + 1)
			ch, err = tb.storage.Get(tb.ctx, ch.SnapshotId)
			if err != nil {
				return "", fmt.Errorf("failed to get snapshot: %w", err)
			}
		}
		current[i] = ch
	}
	// finding common snapshot
	for {
		slices.SortFunc(current, func(a, b StorageChange) int {
			if a.SnapshotId < b.SnapshotId {
				return -1
			}
			if a.SnapshotId > b.SnapshotId {
				return 1
			}
			return 0
		})
		// removing same snapshots
		current = slice.DiscardDuplicatesSortedFunc(current, func(a, b StorageChange) bool {
			return a.SnapshotId == b.SnapshotId
		})
		// if there is only one snapshot left - return it
		if len(current) == 1 {
			return current[0].Id, nil
		}
		// go down one counter
		for i, ch := range current {
			totalCommon.Store(totalCommon.Load() + 1)
			ch, err = tb.storage.Get(tb.ctx, ch.SnapshotId)
			if err != nil {
				return "", fmt.Errorf("failed to get snapshot: %w", err)
			}
			current[i] = ch
		}
	}
}
