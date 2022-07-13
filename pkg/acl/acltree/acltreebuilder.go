package acltree

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
)

type aclTreeBuilder struct {
	cache                map[string]*Change
	identityKeys         map[string]signingkey.SigningPubKey
	signingPubKeyDecoder signingkey.SigningPubKeyDecoder
	tree                 *Tree
	treeStorage          treestorage.TreeStorage

	*changeLoader
}

func newACLTreeBuilder(t treestorage.TreeStorage, decoder signingkey.SigningPubKeyDecoder) *aclTreeBuilder {
	return &aclTreeBuilder{
		signingPubKeyDecoder: decoder,
		treeStorage:          t,
		changeLoader: newChangeLoader(
			t,
			decoder,
			NewACLChange),
	}
}

func (tb *aclTreeBuilder) Init() {
	tb.cache = make(map[string]*Change)
	tb.identityKeys = make(map[string]signingkey.SigningPubKey)
	tb.tree = &Tree{}
	tb.changeLoader.Init(tb.cache, tb.identityKeys)
}

func (tb *aclTreeBuilder) Build() (*Tree, error) {
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
	aclHeads, err := tb.getACLHeads(headsAndOrphans)

	if err != nil {
		return nil, err
	}

	if err = tb.buildTreeFromStart(aclHeads); err != nil {
		return nil, fmt.Errorf("buildTree error: %v", err)
	}
	tb.cache = nil

	return tb.tree, nil
}

func (tb *aclTreeBuilder) buildTreeFromStart(heads []string) (err error) {
	changes, root, err := tb.dfsFromStart(heads)
	if err != nil {
		return err
	}

	tb.tree.AddFast(root)
	tb.tree.AddFast(changes...)
	return
}

func (tb *aclTreeBuilder) dfsFromStart(heads []string) (buf []*Change, root *Change, err error) {
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

func (tb *aclTreeBuilder) getACLHeads(heads []string) (aclTreeHeads []string, err error) {
	for _, head := range heads {
		if slice.FindPos(aclTreeHeads, head) != -1 { // do not scan known heads
			continue
		}
		precedingHeads, err := tb.getPrecedingACLHeads(head)
		if err != nil {
			continue
		}

		for _, aclHead := range precedingHeads {
			if slice.FindPos(aclTreeHeads, aclHead) != -1 {
				continue
			}
			aclTreeHeads = append(aclTreeHeads, aclHead)
		}
	}

	if len(aclTreeHeads) == 0 {
		return nil, fmt.Errorf("no usable ACL heads in tree storage")
	}
	return aclTreeHeads, nil
}

func (tb *aclTreeBuilder) getPrecedingACLHeads(head string) ([]string, error) {
	headChange, err := tb.loadChange(head)
	if err != nil {
		return nil, err
	}

	if headChange.Content.GetAclData() != nil {
		return []string{head}, nil
	} else {
		return headChange.PreviousIds, nil
	}
}
