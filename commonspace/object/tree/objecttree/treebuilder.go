package objecttree

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"golang.org/x/tools/container/intsets"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/slice"
)

var (
	log      = logger.NewNamedSugared("common.commonspace.objecttree")
	ErrEmpty = errors.New("logs empty")
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

func (tb *treeBuilder) Build(opts treeBuilderOpts) (*Tree, error) {
	return tb.build(opts)
}

func (tb *treeBuilder) BuildFull() (*Tree, error) {
	return tb.build(treeBuilderOpts{full: true})
}

func (tb *treeBuilder) build(opts treeBuilderOpts) (tr *Tree, err error) {
	cache := make(map[string]*Change)
	tb.ctx = context.Background()
	for _, ch := range opts.newChanges {
		cache[ch.Id] = ch
	}
	var snapshot string
	if opts.useHeadsSnapshot {
		lowest, err := tb.lowestSnapshots(nil, opts.ourHeads, "")
		if err != nil {
			return nil, err
		}
		if len(lowest) != 1 {
			snapshot, err = tb.commonSnapshot(lowest)
			if err != nil {
				return nil, err
			}
		} else {
			snapshot = lowest[0]
		}
	} else if !opts.full {
		if len(opts.theirSnapshotPath) == 0 {
			if len(opts.ourSnapshotPath) == 0 {
				common, err := tb.storage.CommonSnapshot()
				if err != nil {
					return nil, err
				}
				snapshot = common
			} else {
				our := opts.ourSnapshotPath[len(opts.ourSnapshotPath)-1]
				lowest, err := tb.lowestSnapshots(cache, opts.theirHeads, our)
				if err != nil {
					return nil, err
				}
				if len(lowest) != 1 {
					snapshot, err = tb.commonSnapshot(lowest)
					if err != nil {
						return nil, err
					}
				} else {
					snapshot = lowest[0]
				}
			}
		} else {
			snapshot, err = commonSnapshotForTwoPaths(opts.ourSnapshotPath, opts.theirSnapshotPath)
			if err != nil {
				return nil, err
			}
		}
	} else {
		snapshot = tb.storage.Id()
	}
	snapshotCh, err := tb.storage.Get(tb.ctx, snapshot)
	if err != nil {
		return nil, err
	}
	rawChange := &treechangeproto.RawTreeChangeWithId{}
	var changes []*Change
	err = tb.storage.GetAfterOrder(tb.ctx, snapshotCh.OrderId, func(ctx context.Context, storageChange StorageChange) (shouldContinue bool) {
		rawChange.Id = storageChange.Id
		rawChange.RawChange = storageChange.RawChange
		ch, err := tb.builder.Unmarshall(rawChange, false)
		if err != nil {
			return false
		}
		ch.OrderId = storageChange.OrderId
		ch.SnapshotCounter = storageChange.SnapshotCounter
		changes = append(changes, ch)
		return true
	})
	if err != nil {
		return nil, err
	}
	tr = &Tree{}
	changes = append(changes, opts.newChanges...)
	tr.AddFast(changes...)
	return tr, nil
}

func (tb *treeBuilder) lowestSnapshots(cache map[string]*Change, heads []string, ourSnapshot string) (snapshots []string, err error) {
	var next, current []string
	if cache != nil {
		for _, ch := range heads {
			head, ok := cache[ch]
			if !ok {
				continue
			}
			next = append(next, head.SnapshotId)
		}
	} else {
		for _, head := range heads {
			ch, err := tb.storage.Get(tb.ctx, head)
			if err != nil {
				return nil, err
			}
			next = append(next, ch.SnapshotId)
		}
	}
	slices.Sort(next)
	next = slice.DiscardDuplicatesSorted(next)
	current = make([]string, 0, len(next))
	for len(next) > 0 {
		current = current[:0]
		current = append(current, next...)
		next = next[:0]
		for _, id := range next {
			if ch, ok := cache[id]; ok {
				next = append(next, ch.SnapshotId)
			} else {
				// this is the lowest snapshot from the ones provided
				snapshots = append(snapshots, id)
			}
		}
	}
	if ourSnapshot != "" {
		snapshots = append(snapshots, ourSnapshot)
	}
	slices.Sort(snapshots)
	snapshots = slice.DiscardDuplicatesSorted(snapshots)
	return snapshots, nil
}

func (tb *treeBuilder) commonSnapshot(snapshots []string) (snapshot string, err error) {
	var (
		current       []StorageChange
		lowestCounter = intsets.MaxInt
	)
	// TODO: we should actually check for all changes if they have valid snapshots
	// getting actual snapshots
	for _, id := range snapshots {
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
			ch, err = tb.storage.Get(tb.ctx, ch.SnapshotId)
			if err != nil {
				return "", err
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
			ch, err = tb.storage.Get(tb.ctx, ch.SnapshotId)
			if err != nil {
				return "", err
			}
			current[i] = ch
		}
	}
}
