package document

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/testchanges/testchangepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/node"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/message"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/treecache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var CName = "DocumentService"

var log = logger.NewNamed("documentservice")

type service struct {
	messageService      message.Service
	treeCache           treecache.Service
	account             account.Service
	treeStorageProvider treestorage.Provider
	// to create new documents we need to know all nodes
	nodes []*node.Node
}

type Service interface {
	UpdateDocumentTree(ctx context.Context, id, text string) error
	CreateACLTree(ctx context.Context) (id string, err error)
	CreateDocumentTree(ctx context.Context, aclTreeId string, text string) (id string, err error)
}

func New() app.Component {
	return &service{}
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.account = a.MustComponent(account.CName).(account.Service)
	s.messageService = a.MustComponent(message.CName).(message.Service)
	s.treeCache = a.MustComponent(treecache.CName).(treecache.Service)
	// TODO: add TreeStorageProvider service

	nodesService := a.MustComponent(node.CName).(node.Service)
	s.nodes = nodesService.Nodes()

	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	return nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}

func (s *service) UpdateDocumentTree(ctx context.Context, id, text string) (err error) {
	var (
		ch           *aclpb.RawChange
		header       *treepb.TreeHeader
		snapshotPath []string
		heads        []string
	)
	log.With(zap.String("id", id), zap.String("text", text)).
		Debug("updating document")

	err = s.treeCache.Do(ctx, id, func(obj interface{}) error {
		docTree := obj.(tree.DocTree)
		docTree.Lock()
		defer docTree.Unlock()
		err = s.treeCache.Do(ctx, docTree.Header().AclTreeId, func(obj interface{}) error {
			aclTree := obj.(tree.ACLTree)
			aclTree.RLock()
			defer aclTree.RUnlock()

			content := createAppendTextChange(text)
			_, err := docTree.AddContent(ctx, aclTree, content, false)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}

		id = docTree.ID()
		heads = docTree.Heads()
		header = docTree.Header()
		snapshotPath = docTree.SnapshotPath()
		return nil
	})
	if err != nil {
		return err
	}
	log.With(
		zap.String("id", id),
		zap.Strings("heads", heads),
		zap.String("header", header.String())).
		Debug("document updated in the database")

	return s.messageService.SendToSpaceAsync("", syncproto.WrapHeadUpdate(&syncproto.SyncHeadUpdate{
		Heads:        heads,
		Changes:      []*aclpb.RawChange{ch},
		TreeId:       id,
		SnapshotPath: snapshotPath,
		TreeHeader:   header,
	}))
}

func (s *service) CreateACLTree(ctx context.Context) (id string, err error) {
	acc := s.account.Account()
	var (
		ch           *aclpb.RawChange
		header       *treepb.TreeHeader
		snapshotPath []string
		heads        []string
	)

	t, err := tree.CreateNewTreeStorageWithACL(acc, func(builder tree.ACLChangeBuilder) error {
		err := builder.UserAdd(acc.Identity, acc.EncKey.GetPublic(), aclpb.ACLChange_Admin)
		if err != nil {
			return err
		}
		// adding all predefined nodes to the document as admins
		for _, n := range s.nodes {
			err = builder.UserAdd(n.SigningKeyString, n.EncryptionKey, aclpb.ACLChange_Admin)
			if err != nil {
				return err
			}
		}
		return nil
	}, s.treeStorageProvider.CreateTreeStorage)

	id, err = t.TreeID()
	if err != nil {
		return "", err
	}

	header, err = t.Header()
	if err != nil {
		return "", err
	}

	heads = []string{header.FirstChangeId}
	snapshotPath = []string{header.FirstChangeId}
	ch, err = t.GetChange(ctx, header.FirstChangeId)
	if err != nil {
		return "", err
	}

	if err != nil {
		return "", err
	}

	err = s.messageService.SendToSpaceAsync("", syncproto.WrapHeadUpdate(&syncproto.SyncHeadUpdate{
		Heads:        heads,
		Changes:      []*aclpb.RawChange{ch},
		TreeId:       id,
		SnapshotPath: snapshotPath,
		TreeHeader:   header,
	}))
	return id, nil
}

func (s *service) CreateDocumentTree(ctx context.Context, aclTreeId string, text string) (id string, err error) {
	acc := s.account.Account()
	var (
		ch           *aclpb.RawChange
		header       *treepb.TreeHeader
		snapshotPath []string
		heads        []string
	)
	err = s.treeCache.Do(ctx, aclTreeId, func(obj interface{}) error {
		t := obj.(tree.ACLTree)
		t.RLock()
		defer t.RUnlock()

		content := createInitialTextChange(text)
		doc, err := tree.CreateNewTreeStorage(acc, t, content, s.treeStorageProvider.CreateTreeStorage)
		if err != nil {
			return err
		}

		id, err = doc.TreeID()
		if err != nil {
			return err
		}

		header, err = doc.Header()
		if err != nil {
			return err
		}

		heads = []string{header.FirstChangeId}
		snapshotPath = []string{header.FirstChangeId}
		ch, err = doc.GetChange(ctx, header.FirstChangeId)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	log.With(zap.String("id", id), zap.String("text", text)).
		Debug("creating document")

	err = s.messageService.SendToSpaceAsync("", syncproto.WrapHeadUpdate(&syncproto.SyncHeadUpdate{
		Heads:        heads,
		Changes:      []*aclpb.RawChange{ch},
		TreeId:       id,
		SnapshotPath: snapshotPath,
		TreeHeader:   header,
	}))
	if err != nil {
		return "", err
	}
	return id, err
}

func createInitialTextChange(text string) proto.Marshaler {
	return &testchangepb.PlainTextChangeData{
		Content: []*testchangepb.PlainTextChangeContent{
			createAppendTextChangeContent(text),
		},
		Snapshot: &testchangepb.PlainTextChangeSnapshot{Text: text},
	}
}

func createAppendTextChange(text string) proto.Marshaler {
	return &testchangepb.PlainTextChangeData{
		Content: []*testchangepb.PlainTextChangeContent{
			createAppendTextChangeContent(text),
		},
	}
}

func createAppendTextChangeContent(text string) *testchangepb.PlainTextChangeContent {
	return &testchangepb.PlainTextChangeContent{
		Value: &testchangepb.PlainTextChangeContentValueOfTextAppend{
			TextAppend: &testchangepb.PlainTextChangeTextAppend{
				Text: text,
			},
		},
	}
}
