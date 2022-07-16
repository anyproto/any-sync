package acltree

import (
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
)

var (
	log      = logger.NewNamed("acltree").Sugar()
	ErrEmpty = errors.New("logs empty")
)

type treeBuilder struct {
	cache                map[string]*Change
	identityKeys         map[string]signingkey.PubKey
	signingPubKeyDecoder signingkey.PubKeyDecoder
	tree                 *Tree
	treeStorage          treestorage.TreeStorage

	*changeLoader
}

func newTreeBuilder(t treestorage.TreeStorage, decoder signingkey.PubKeyDecoder) *treeBuilder {
	return &treeBuilder{
		signingPubKeyDecoder: decoder,
		treeStorage:          t,
		changeLoader: newChangeLoader(
			t,
			decoder,
			NewChange),
	}
}

func (tb *treeBuilder) Init() {
	tb.cache = make(map[string]*Change)
	tb.identityKeys = make(map[string]signingkey.PubKey)
	tb.tree = &Tree{}
	tb.changeLoader.Init(tb.cache, tb.identityKeys)
}

func (tb *treeBuilder) Build(fromStart bool) (*Tree, error) {
	var headsAndOrphans []string
	orphans, err := tb.treeStorage.Orphans()
	if err != nil {
		return nil, err
	}
	heads, err := tb.treeStorage.Heads()
	if err != nil {
		return nil, err
	}
	headsAndOrphans = append(headsAndOrphans, orphans...)
	headsAndOrphans = append(headsAndOrphans, heads...)

	if fromStart {
		if err := tb.buildTreeFromStart(headsAndOrphans); err != nil {
			return nil, fmt.Errorf("buildTree error: %v", err)
		}
	} else {
		breakpoint, err := tb.findBreakpoint(headsAndOrphans)
		if err != nil {
			return nil, fmt.Errorf("findBreakpoint error: %v", err)
		}

		if err = tb.buildTree(headsAndOrphans, breakpoint); err != nil {
			return nil, fmt.Errorf("buildTree error: %v", err)
		}
	}

	tb.cache = nil

	return tb.tree, nil
}

func (tb *treeBuilder) buildTreeFromStart(heads []string) (err error) {
	changes, root, err := tb.dfsFromStart(heads)
	if err != nil {
		return err
	}

	tb.tree.AddFast(root)
	tb.tree.AddFast(changes...)
	return
}

func (tb *treeBuilder) dfsFromStart(heads []string) (buf []*Change, root *Change, err error) {
	var possibleRoots []*Change
	stack := make([]string, len(heads), len(heads)*2)
	copy(stack, heads)

	buf = make([]*Change, 0, len(stack)*2)
	uniqMap := make(map[string]struct{})
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, exists := uniqMap[id]; exists {
			continue
		}

		ch, err := tb.loadChange(id)
		if err != nil {
			continue
		}

		uniqMap[id] = struct{}{}
		buf = append(buf, ch)

		for _, prev := range ch.PreviousIds {
			stack = append(stack, prev)
		}
		if len(ch.PreviousIds) == 0 {
			possibleRoots = append(possibleRoots, ch)
		}
	}
	header, err := tb.treeStorage.Header()
	if err != nil {
		return nil, nil, err
	}
	for _, r := range possibleRoots {
		if r.Id == header.FirstChangeId {
			return buf, r, nil
		}
	}

	return nil, nil, fmt.Errorf("could not find root change")
}

func (tb *treeBuilder) buildTree(heads []string, breakpoint string) (err error) {
	ch, err := tb.loadChange(breakpoint)
	if err != nil {
		return
	}
	tb.tree.AddFast(ch)
	changes, err := tb.dfs(heads, breakpoint, tb.loadChange)

	tb.tree.AddFast(changes...)
	return
}

func (tb *treeBuilder) dfs(
	heads []string,
	breakpoint string,
	load func(string) (*Change, error)) (buf []*Change, err error) {
	stack := make([]string, len(heads), len(heads)*2)
	copy(stack, heads)

	buf = make([]*Change, 0, len(stack)*2)
	uniqMap := map[string]struct{}{breakpoint: {}}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, exists := uniqMap[id]; exists {
			continue
		}

		ch, err := load(id)
		if err != nil {
			continue
		}

		uniqMap[id] = struct{}{}
		buf = append(buf, ch)

		for _, prev := range ch.PreviousIds {
			stack = append(stack, prev)
		}
	}
	return buf, nil
}

func (tb *treeBuilder) findBreakpoint(heads []string) (breakpoint string, err error) {
	var (
		ch          *Change
		snapshotIds []string
	)
	for _, head := range heads {
		if ch, err = tb.loadChange(head); err != nil {
			return
		}
		shId := ch.SnapshotId
		if ch.IsSnapshot {
			shId = ch.Id
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
		// TODO: add more sophisticated checks in Change for snapshots
		return !ch.IsSnapshot
	}

	// TODO: can we even have empty snapshots?
	// prefer not empty snapshot
	if isEmptySnapshot(ch1) && !isEmptySnapshot(ch2) {
		log.Warnf("changes build Tree: prefer %s(not empty) over %s(empty)", s2, s1)
		return s2, nil
	} else if isEmptySnapshot(ch2) && !isEmptySnapshot(ch1) {
		log.Warnf("changes build Tree: prefer %s(not empty) over %s(empty)", s1, s2)
		return s1, nil
	}

	// TODO: add virtual change mechanics
	// unexpected behavior - just return lesser id
	if s1 < s2 {
		log.Warnf("changes build Tree: prefer %s (%s<%s)", s1, s1, s2)
		return s1, nil
	}
	log.Warnf("changes build Tree: prefer %s (%s<%s)", s2, s2, s1)

	return s2, nil
}
