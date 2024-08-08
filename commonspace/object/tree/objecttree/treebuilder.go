package objecttree

import (
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/util/slice"
)

var (
	log      = logger.NewNamedSugared("common.commonspace.objecttree")
	ErrEmpty = errors.New("logs empty")
)

type treeBuilder struct {
	treeStorage treestorage.TreeStorage
	builder     ChangeBuilder
	loader      *rawChangeLoader

	cache            map[string]*Change
	tree             *Tree
	keepInMemoryData bool

	// buffers
	idStack    []string
	loadBuffer []*Change
}

func newTreeBuilder(keepData bool, storage treestorage.TreeStorage, builder ChangeBuilder, loader *rawChangeLoader) *treeBuilder {
	return &treeBuilder{
		treeStorage:      storage,
		builder:          builder,
		loader:           loader,
		keepInMemoryData: keepData,
	}
}

func (tb *treeBuilder) Reset() {
	tb.cache = make(map[string]*Change)
	tb.tree = &Tree{}
}

func (tb *treeBuilder) Build(theirHeads []string, newChanges []*Change) (*Tree, error) {
	heads, err := tb.treeStorage.Heads()
	if err != nil {
		return nil, err
	}
	return tb.build(heads, theirHeads, newChanges)
}

func (tb *treeBuilder) BuildFull() (*Tree, error) {
	defer func() {
		tb.cache = make(map[string]*Change)
	}()
	tb.cache = make(map[string]*Change)
	heads, err := tb.treeStorage.Heads()
	if err != nil {
		return nil, err
	}
	err = tb.buildTree(heads, tb.treeStorage.Id())
	if err != nil {
		return nil, err
	}
	return tb.tree, nil
}

func (tb *treeBuilder) build(heads []string, theirHeads []string, newChanges []*Change) (*Tree, error) {
	defer func() {
		tb.cache = make(map[string]*Change)
	}()

	var proposedHeads []string
	tb.cache = make(map[string]*Change)

	// TODO: we can actually get this from tree (though not sure, that there would always be
	//  an invariant where the tree has the closest common snapshot of heads)
	//  so if optimization is critical we can change this to inject from tree directly,
	//  but then we have to be sure that invariant stays true
	oldBreakpoint, err := tb.findBreakpoint(heads, true)
	if err != nil {
		// this should never error out, because otherwise we have broken data
		return nil, fmt.Errorf("findBreakpoint error: %v", err)
	}

	if len(theirHeads) > 0 {
		proposedHeads = append(proposedHeads, theirHeads...)
	}
	for _, ch := range newChanges {
		if len(theirHeads) == 0 {
			// in this case we don't know what new heads are, so every change can be head
			proposedHeads = append(proposedHeads, ch.Id)
		}
		tb.cache[ch.Id] = ch
	}

	// getting common snapshot for new heads
	breakpoint, err := tb.findBreakpoint(proposedHeads, false)
	if err != nil {
		breakpoint = oldBreakpoint
	} else {
		breakpoint, err = tb.findCommonForTwoSnapshots(oldBreakpoint, breakpoint)
		if err != nil {
			breakpoint = oldBreakpoint
		}
	}
	proposedHeads = append(proposedHeads, heads...)

	log.With(zap.Strings("heads", proposedHeads), zap.String("id", tb.treeStorage.Id())).Debug("building tree")
	if err = tb.buildTree(proposedHeads, breakpoint); err != nil {
		return nil, fmt.Errorf("buildTree error: %v", err)
	}

	return tb.tree, nil
}

func (tb *treeBuilder) buildTree(heads []string, breakpoint string) (err error) {
	ch, err := tb.loadChange(breakpoint)
	if err != nil {
		return
	}
	changes := tb.dfs(heads, breakpoint)
	tb.tree.AddFast(ch)
	tb.tree.AddFast(changes...)
	return
}

func (tb *treeBuilder) dfs(heads []string, breakpoint string) []*Change {
	// initializing buffers
	tb.idStack = tb.idStack[:0]
	tb.loadBuffer = tb.loadBuffer[:0]

	// updating map
	uniqMap := map[string]struct{}{breakpoint: {}}

	// preparing dfs
	tb.idStack = append(tb.idStack, heads...)

	// dfs
	for len(tb.idStack) > 0 {
		id := tb.idStack[len(tb.idStack)-1]
		tb.idStack = tb.idStack[:len(tb.idStack)-1]
		if _, exists := uniqMap[id]; exists {
			continue
		}

		ch, err := tb.loadChange(id)
		if err != nil {
			continue
		}

		uniqMap[id] = struct{}{}
		tb.loadBuffer = append(tb.loadBuffer, ch)

		for _, prev := range ch.PreviousIds {
			if _, exists := uniqMap[prev]; exists {
				continue
			}
			tb.idStack = append(tb.idStack, prev)
		}
	}
	return tb.loadBuffer
}

func (tb *treeBuilder) loadChange(id string) (ch *Change, err error) {
	if ch, ok := tb.cache[id]; ok {
		return ch, nil
	}

	change, err := tb.loader.loadAppendRaw(id)
	if err != nil {
		return nil, err
	}

	ch, err = tb.builder.Unmarshall(change, true)
	if err != nil {
		return nil, err
	}
	// TODO: see if we can delete this
	if !tb.keepInMemoryData {
		ch.Data = nil
	}

	tb.cache[id] = ch
	return ch, nil
}

func (tb *treeBuilder) findBreakpoint(heads []string, noError bool) (breakpoint string, err error) {
	var (
		ch          *Change
		snapshotIds []string
	)
	for _, head := range heads {
		if ch, err = tb.loadChange(head); err != nil {
			if noError {
				return
			}

			log.With(zap.String("head", head), zap.Error(err)).Debug("couldn't find head")
			continue
		}

		shId := ch.SnapshotId
		if ch.IsSnapshot {
			shId = ch.Id
		} else {
			_, err = tb.loadChange(shId)
			if err != nil {
				if noError {
					return
				}

				log.With(zap.String("snapshot id", shId), zap.Error(err)).Debug("couldn't find head's snapshot")
				continue
			}
		}
		if slice.FindPos(snapshotIds, shId) == -1 {
			snapshotIds = append(snapshotIds, shId)
		}
	}
	return tb.findCommonSnapshot(snapshotIds)
}

func (tb *treeBuilder) findCommonSnapshot(snapshotIds []string) (snapshotId string, err error) {
	if len(snapshotIds) == 1 {
		return snapshotIds[0], nil
	} else if len(snapshotIds) == 0 {
		return "", fmt.Errorf("snapshots not found")
	}

	// TODO: use divide and conquer to find the snapshot, then we will have only logN findCommonForTwoSnapshots calls
	for len(snapshotIds) > 1 {
		l := len(snapshotIds)
		shId, e := tb.findCommonForTwoSnapshots(snapshotIds[l-2], snapshotIds[l-1])
		if e != nil {
			return "", e
		}
		snapshotIds[l-2] = shId
		snapshotIds = snapshotIds[:l-1]
	}
	return snapshotIds[0], nil
}

func (tb *treeBuilder) findCommonForTwoSnapshots(s1, s2 string) (s string, err error) {
	// fast cases
	if s1 == s2 {
		return s1, nil
	}
	ch1, err := tb.loadChange(s1)
	if err != nil {
		return "", err
	}
	if ch1.SnapshotId == s2 {
		return s2, nil
	}
	ch2, err := tb.loadChange(s2)
	if err != nil {
		return "", err
	}
	if ch2.SnapshotId == s1 {
		return s1, nil
	}
	if ch1.SnapshotId == ch2.SnapshotId && ch1.SnapshotId != "" {
		return ch1.SnapshotId, nil
	}
	// traverse
	var t1 = make([]string, 0, 5)
	var t2 = make([]string, 0, 5)
	t1 = append(t1, ch1.Id, ch1.SnapshotId)
	t2 = append(t2, ch2.Id, ch2.SnapshotId)
	for {
		lid1 := t1[len(t1)-1]
		if lid1 != "" {
			l1, e := tb.loadChange(lid1)
			if e != nil {
				return "", e
			}
			if l1.SnapshotId != "" {
				if slice.FindPos(t2, l1.SnapshotId) != -1 {
					return l1.SnapshotId, nil
				}
			}
			t1 = append(t1, l1.SnapshotId)
		}
		lid2 := t2[len(t2)-1]
		if lid2 != "" {
			l2, e := tb.loadChange(t2[len(t2)-1])
			if e != nil {
				return "", e
			}
			if l2.SnapshotId != "" {
				if slice.FindPos(t1, l2.SnapshotId) != -1 {
					return l2.SnapshotId, nil
				}
			}
			t2 = append(t2, l2.SnapshotId)
		}
		if lid1 == "" && lid2 == "" {
			break
		}
	}

	log.Warnf("changes build Tree: possible versions split")

	// prefer not first snapshot
	if len(ch1.PreviousIds) == 0 && len(ch2.PreviousIds) > 0 {
		log.Warnf("changes build Tree: prefer %s(%d prevIds) over %s(%d prevIds)", s2, len(ch2.PreviousIds), s1, len(ch1.PreviousIds))
		return s2, nil
	} else if len(ch1.PreviousIds) > 0 && len(ch2.PreviousIds) == 0 {
		log.Warnf("changes build Tree: prefer %s(%d prevIds) over %s(%d prevIds)", s1, len(ch1.PreviousIds), s2, len(ch2.PreviousIds))
		return s1, nil
	}

	isEmptySnapshot := func(ch *Change) bool {
		return !ch.IsSnapshot
	}

	// prefer not empty snapshot
	if isEmptySnapshot(ch1) && !isEmptySnapshot(ch2) {
		log.Warnf("changes build Tree: prefer %s(not empty) over %s(empty)", s2, s1)
		return s2, nil
	} else if isEmptySnapshot(ch2) && !isEmptySnapshot(ch1) {
		log.Warnf("changes build Tree: prefer %s(not empty) over %s(empty)", s1, s2)
		return s1, nil
	}

	// unexpected behavior - just return lesser id
	if s1 < s2 {
		log.Warnf("changes build Tree: prefer %s (%s<%s)", s1, s1, s2)
		return s1, nil
	}
	log.Warnf("changes build Tree: prefer %s (%s<%s)", s2, s2, s1)

	return s2, nil
}
