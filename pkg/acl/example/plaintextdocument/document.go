package plaintextdocument

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	aclpb "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/acltree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/testchanges/testchangepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"

	"github.com/gogo/protobuf/proto"
)

type PlainTextDocument interface {
	Text() string
	AddText(ctx context.Context, text string) error
}

// TODO: this struct is not thread-safe, so use it wisely :-)
type plainTextDocument struct {
	heads   []string
	aclTree acltree.ACLTree
	state   *DocumentState
}

func (p *plainTextDocument) Text() string {
	if p.state != nil {
		return p.state.Text
	}
	return ""
}

func (p *plainTextDocument) AddText(ctx context.Context, text string) error {
	_, err := p.aclTree.AddContent(ctx, func(builder acltree.ChangeBuilder) error {
		builder.AddChangeContent(
			&testchangepb.PlainTextChange_Data{
				Content: []*testchangepb.PlainTextChange_Content{
					createAppendTextChangeContent(text),
				},
			})
		return nil
	})
	return err
}

func (p *plainTextDocument) Update(tree acltree.ACLTree) {
	p.aclTree = tree
	var err error
	defer func() {
		if err != nil {
			fmt.Println("rebuild has returned error:", err)
		}
	}()

	prevHeads := p.heads
	p.heads = tree.Heads()
	startId := prevHeads[0]
	tree.IterateFrom(startId, func(change *acltree.Change) (isContinue bool) {
		if change.Id == startId {
			return true
		}
		if change.DecryptedDocumentChange != nil {
			p.state, err = p.state.ApplyChange(change.DecryptedDocumentChange, change.Id)
			if err != nil {
				return false
			}
		}
		return true
	})
}

func (p *plainTextDocument) Rebuild(tree acltree.ACLTree) {
	p.aclTree = tree
	p.heads = tree.Heads()
	var startId string
	var err error
	defer func() {
		if err != nil {
			fmt.Println("rebuild has returned error:", err)
		}
	}()

	rootChange := tree.Root()

	if rootChange.DecryptedDocumentChange == nil {
		err = fmt.Errorf("root doesn't have decrypted change")
		return
	}

	state, err := BuildDocumentStateFromChange(rootChange.DecryptedDocumentChange, rootChange.Id)
	if err != nil {
		return
	}

	startId = rootChange.Id
	tree.Iterate(func(change *acltree.Change) (isContinue bool) {
		if startId == change.Id {
			return true
		}
		if change.DecryptedDocumentChange != nil {
			state, err = state.ApplyChange(change.DecryptedDocumentChange, change.Id)
			if err != nil {
				return false
			}
		}
		return true
	})
	if err != nil {
		return
	}
	p.state = state
}

func NewInMemoryPlainTextDocument(acc *account.AccountData, text string) (PlainTextDocument, error) {
	return NewPlainTextDocument(acc, treestorage.NewInMemoryTreeStorage, text)
}

func NewPlainTextDocument(
	acc *account.AccountData,
	create treestorage.CreatorFunc,
	text string) (PlainTextDocument, error) {
	changeBuilder := func(builder acltree.ChangeBuilder) error {
		err := builder.UserAdd(acc.Identity, acc.EncKey.GetPublic(), aclpb.ACLChange_Admin)
		if err != nil {
			return err
		}
		builder.AddChangeContent(createInitialChangeContent(text))
		return nil
	}
	t, err := acltree.CreateNewTreeStorageWithACL(
		acc,
		changeBuilder,
		create)
	if err != nil {
		return nil, err
	}

	doc := &plainTextDocument{
		heads:   nil,
		aclTree: nil,
		state:   nil,
	}
	tree, err := acltree.BuildACLTree(t, acc, doc)
	if err != nil {
		return nil, err
	}
	doc.aclTree = tree
	return doc, nil
}

func createInitialChangeContent(text string) proto.Marshaler {
	return &testchangepb.PlainTextChange_Data{
		Content: []*testchangepb.PlainTextChange_Content{
			createAppendTextChangeContent(text),
		},
		Snapshot: &testchangepb.PlainTextChange_Snapshot{Text: text},
	}
}

func createAppendTextChangeContent(text string) *testchangepb.PlainTextChange_Content {
	return &testchangepb.PlainTextChange_Content{
		Value: &testchangepb.PlainTextChange_Content_TextAppend{
			TextAppend: &testchangepb.PlainTextChange_TextAppend{
				Text: text,
			},
		},
	}
}
