package textdocument

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	testchanges "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/testutils/testchanges/proto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"github.com/gogo/protobuf/proto"
)

type TextDocument interface {
	tree.ObjectTree
	AddText(text string) error
	Text() (string, error)
	TreeDump() string
	Close() error
}

type textDocument struct {
	tree.ObjectTree
	account account.Service
}

func CreateTextDocument(
	ctx context.Context,
	space commonspace.Space,
	account account.Service,
	listener updatelistener.UpdateListener) (doc TextDocument, err error) {
	payload := tree.ObjectTreeCreatePayload{
		SignKey:  account.Account().SignKey,
		SpaceId:  space.Id(),
		Identity: account.Account().Identity,
	}
	t, err := space.CreateTree(ctx, payload, listener)
	if err != nil {
		return
	}

	return &textDocument{
		ObjectTree: t,
		account:    account,
	}, nil
}

func NewTextDocument(ctx context.Context, space commonspace.Space, id string, listener updatelistener.UpdateListener, account account.Service) (doc TextDocument, err error) {
	t, err := space.BuildTree(ctx, id, listener)
	if err != nil {
		return
	}
	return &textDocument{
		ObjectTree: t,
		account:    account,
	}, nil
}

func (t *textDocument) AddText(text string) (err error) {
	content := &testchanges.TextContent_TextAppend{
		TextAppend: &testchanges.TextAppend{Text: text},
	}
	change := &testchanges.TextData{
		Content: []*testchanges.TextContent{
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
	_, err = t.AddContent(context.Background(), tree.SignableChangeContent{
		Data:       res,
		Key:        t.account.Account().SignKey,
		Identity:   t.account.Account().Identity,
		IsSnapshot: false,
	})
	return
}

func (t *textDocument) Text() (text string, err error) {
	t.RLock()
	defer t.RUnlock()

	err = t.Iterate(
		func(decrypted []byte) (any, error) {
			textChange := &testchanges.TextData{}
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
		}, func(change *tree.Change) bool {
			return true
		})
	return
}

func (t *textDocument) TreeDump() string {
	return t.TreeDump()
}

func (t *textDocument) Close() error {
	return nil
}
