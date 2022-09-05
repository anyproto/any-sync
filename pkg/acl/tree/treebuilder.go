package tree

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"go.uber.org/zap"
	"time"
)

var (
	log      = logger.NewNamed("acltree").Sugar()
	ErrEmpty = errors.New("logs empty")
)

type treeBuilder struct {
	cache       map[string]*Change
	kch         *keychain
	tree        *Tree
	treeStorage storage.TreeStorage

	// buffers
	idStack    []string
	loadBuffer []*Change
}

func newTreeBuilder(t storage.TreeStorage) *treeBuilder {
	return &treeBuilder{
		treeStorage: t,
	}
}

func (tb *treeBuilder) Init(kch *keychain) {
	tb.cache = make(map[string]*Change)
	tb.kch = kch
	tb.tree = &Tree{}
}

func (tb *treeBuilder) Build(newChanges []*Change) (*Tree, error) {
	var headsAndNewChanges []string
	heads, err := tb.treeStorage.Heads()
	if err != nil {
		return nil, err
	}

	headsAndNewChanges = append(headsAndNewChanges, heads...)
	tb.cache = make(map[string]*Change)
	for _, ch := range newChanges {
		headsAndNewChanges = append(headsAndNewChanges, ch.Id)
		tb.cache[ch.Id] = ch
	}

	log.With(zap.Strings("heads", heads)).Debug("building tree")
	breakpoint, err := tb.findBreakpoint(headsAndNewChanges)
	if err != nil {
		return nil, fmt.Errorf("findBreakpoint error: %v", err)
	}

	if err = tb.buildTree(headsAndNewChanges, breakpoint); err != nil {
		return nil, fmt.Errorf("buildTree error: %v", err)
	}

	return tb.tree, nil
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
	tb.idStack = tb.idStack[:0]
	tb.loadBuffer = tb.loadBuffer[:0]
	buf = tb.loadBuffer

	var (
		stack   = tb.idStack
		uniqMap = map[string]struct{}{breakpoint: {}}
	)

	copy(stack, heads)

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
			if _, exists := uniqMap[id]; exists {
				continue
			}
			stack = append(stack, prev)
		}
	}
	return buf, nil
}

func (tb *treeBuilder) loadChange(id string) (ch *Change, err error) {
	if ch, ok := tb.cache[id]; ok {
		return ch, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	change, err := tb.treeStorage.GetRawChange(ctx, id)
	if err != nil {
		return nil, err
	}

	ch, err = NewVerifiedChangeFromRaw(change, tb.identityKeys, tb.signingPubKeyDecoder)
	if err != nil {
		return nil, err
	}

	tb.cache[id] = ch
	return ch, nil
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
