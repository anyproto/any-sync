package textdocument

import (
	"context"
	textchange "github.com/anytypeio/go-anytype-infrastructure-experiments/client/document/textchangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/objecttree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/tree/treestorage"
	"github.com/gogo/protobuf/proto"
)

type TextDocument interface {
	objecttree.ObjectTree
	InnerTree() objecttree.ObjectTree
	AddText(text string, isSnapshot bool) (string, string, error)
	Text() (string, error)
	TreeDump() string
	Close() error
}

type textDocument struct {
	objecttree.ObjectTree
	account accountservice.Service
}

func CreateTextDocument(
	ctx context.Context,
	space commonspace.Space,
	payload treestorage.TreeStorageCreatePayload,
	listener updatelistener.UpdateListener,
	account accountservice.Service) (doc TextDocument, err error) {
	t, err := space.PutTree(ctx, payload, listener)
	if err != nil {
		return
	}
	return &textDocument{
		ObjectTree: t,
		account:    account,
	}, nil
}

func NewTextDocument(ctx context.Context, space commonspace.Space, id string, listener updatelistener.UpdateListener, account accountservice.Service) (doc TextDocument, err error) {
	t, err := space.BuildTree(ctx, id, listener)
	if err != nil {
		return
	}
	return &textDocument{
		ObjectTree: t,
		account:    account,
	}, nil
}

func (t *textDocument) InnerTree() objecttree.ObjectTree {
	return t.ObjectTree
}

func (t *textDocument) AddText(text string, isSnapshot bool) (root, head string, err error) {
	content := &textchange.TextContent_TextAppend{
		TextAppend: &textchange.TextAppend{Text: text},
	}
	change := &textchange.TextData{
		Content: []*textchange.TextContent{
			{content},
		},
		Snapshot: nil,
	}
	res, err := change.Marshal()
	if err != nil {
		return
	}
	t.Lock()
	defer t.Unlock()
	addRes, err := t.AddContent(context.Background(), objecttree.SignableChangeContent{
		Data:       res,
		Key:        t.account.Account().SignKey,
		Identity:   t.account.Account().Identity,
		IsSnapshot: isSnapshot,
	})
	if err != nil {
		return
	}
	root = t.Root().Id
	head = addRes.Heads[0]
	return
}

func (t *textDocument) Text() (text string, err error) {
	t.RLock()
	defer t.RUnlock()

	err = t.IterateRoot(
		func(decrypted []byte) (any, error) {
			textChange := &textchange.TextData{}
			err = proto.Unmarshal(decrypted, textChange)
			if err != nil {
				return nil, err
			}
			for _, cnt := range textChange.Content {
				if cnt.GetTextAppend() != nil {
					text += cnt.GetTextAppend().Text
				}
			}
			return textChange, nil
		}, func(change *objecttree.Change) bool {
			return true
		})
	return
}

func (t *textDocument) TreeDump() string {
	return t.TreeDump()
}
