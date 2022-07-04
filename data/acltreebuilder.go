package data

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadmodels"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"github.com/gogo/protobuf/proto"
	"github.com/textileio/go-threads/core/thread"
	"time"
)

type ACLTreeBuilder struct {
	cache                map[string]*Change
	identityKeys         map[string]threadmodels.SigningPubKey
	signingPubKeyDecoder threadmodels.SigningPubKeyDecoder
	tree                 *Tree
	thread               threadmodels.Thread
}

func NewACLTreeBuilder(t threadmodels.Thread, decoder threadmodels.SigningPubKeyDecoder) *ACLTreeBuilder {
	return &ACLTreeBuilder{
		cache:                make(map[string]*Change),
		identityKeys:         make(map[string]threadmodels.SigningPubKey),
		signingPubKeyDecoder: decoder,
		tree:                 &Tree{}, // TODO: add NewTree method
		thread:               t,
	}
}

func (tb *ACLTreeBuilder) loadChange(id string) (ch *Change, err error) {
	if ch, ok := tb.cache[id]; ok {
		return ch, nil
	}

	// TODO: Add virtual changes logic
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	change, err := tb.thread.GetChange(ctx, id)
	if err != nil {
		return nil, err
	}

	aclChange := new(pb.ACLChange)

	// TODO: think what should we do with such cases, because this can be used by attacker to break our tree
	if err = proto.Unmarshal(change.Payload, aclChange); err != nil {
		return
	}
	var verified bool
	verified, err = tb.verify(aclChange.Identity, change.Payload, change.Signature)
	if err != nil {
		return
	}
	if !verified {
		err = fmt.Errorf("the signature of the payload cannot be verified")
		return
	}

	ch, err = NewACLChange(id, aclChange)
	tb.cache[id] = ch

	return ch, nil
}

func (tb *ACLTreeBuilder) verify(identity string, payload, signature []byte) (isVerified bool, err error) {
	identityKey, exists := tb.identityKeys[identity]
	if !exists {
		identityKey, err = tb.signingPubKeyDecoder.DecodeFromString(identity)
		if err != nil {
			return
		}
		tb.identityKeys[identity] = identityKey
	}
	return identityKey.Verify(payload, signature)
}

func (tb *ACLTreeBuilder) Build() (*Tree, error) {
	heads := tb.thread.Heads()
	aclHeads, err := tb.getACLHeads(heads)
	if err != nil {
		return nil, err
	}

	if err = tb.buildTreeFromStart(aclHeads); err != nil {
		return nil, fmt.Errorf("buildTree error: %v", err)
	}
	tb.cache = nil

	return tb.tree, nil
}

func (tb *ACLTreeBuilder) buildTreeFromStart(heads []string) (err error) {
	changes, possibleRoots, err := tb.dfsFromStart(heads)
	if len(possibleRoots) == 0 {
		return fmt.Errorf("cannot have tree without root")
	}
	root, err := tb.getRoot(possibleRoots)
	if err != nil {
		return err
	}

	tb.tree.AddFast(root)
	tb.tree.AddFast(changes...)
	return
}

func (tb *ACLTreeBuilder) dfsFromStart(heads []string) (buf []*Change, possibleRoots []*Change, err error) {
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
	return buf, possibleRoots, nil
}

func (tb *ACLTreeBuilder) getRoot(possibleRoots []*Change) (*Change, error) {
	threadId, err := thread.Decode(tb.thread.ID())
	if err != nil {
		return nil, err
	}

	for _, r := range possibleRoots {
		id := r.Content.Identity
		sk, err := tb.signingPubKeyDecoder.DecodeFromString(id)
		if err != nil {
			continue
		}

		res, err := threadmodels.VerifyACLThreadID(sk, threadId)
		if err != nil {
			continue
		}

		if res {
			return r, nil
		}
	}
	return nil, fmt.Errorf("could not find any root")
}

func (tb *ACLTreeBuilder) getACLHeads(heads []string) (aclTreeHeads []string, err error) {
	for _, head := range heads {
		if slice.FindPos(aclTreeHeads, head) != -1 { // do not scan known heads
			continue
		}
		precedingHeads, err := tb.getPrecedingACLHeads(head)
		if err != nil {
			return nil, err
		}

		for _, aclHead := range precedingHeads {
			if slice.FindPos(aclTreeHeads, aclHead) != -1 {
				continue
			}
			aclTreeHeads = append(aclTreeHeads, aclHead)
		}
	}

	if len(aclTreeHeads) == 0 {
		return nil, fmt.Errorf("no usable ACL heads in thread")
	}
	return aclTreeHeads, nil
}

func (tb *ACLTreeBuilder) getPrecedingACLHeads(head string) ([]string, error) {
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
