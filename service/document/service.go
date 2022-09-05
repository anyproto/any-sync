package document

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	testchanges "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/testchanges/proto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/node"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/sync/message"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/treecache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var CName = "DocumentService"

var log = logger.NewNamed("documentservice")

type service struct {
	messageService message.Service
	treeCache      treecache.Service
	account        account.Service
	storage        storage.Service
	// to create new documents we need to know all nodes
	nodes []*node.Node
}

type Service interface {
	UpdateDocumentTree(ctx context.Context, id, text string) error
	CreateDocumentTree(ctx context.Context, aclTreeId string, text string) (id string, err error)
}

func New() app.Component {
	return &service{}
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.account = a.MustComponent(account.CName).(account.Service)
	s.messageService = a.MustComponent(message.CName).(message.Service)
	s.treeCache = a.MustComponent(treecache.CName).(treecache.Service)
	s.storage = a.MustComponent(storage.CName).(storage.Service)

	nodesService := a.MustComponent(node.CName).(node.Service)
	s.nodes = nodesService.Nodes()

	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	syncData := s.storage.ImportedACLSyncData()

	// we could have added a timeout or some additional logic,
	// but let's just use the ACL id of the latest started node :-)
	return s.messageService.SendToSpaceAsync("", syncproto.WrapACLList(
		&syncproto.SyncACLList{Records: syncData.Records},
		syncData.Header,
		syncData.Id,
	))
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}

func (s *service) UpdateDocumentTree(ctx context.Context, id, text string) (err error) {
	var (
		ch           *aclpb.RawChange
		header       *aclpb.Header
		snapshotPath []string
		heads        []string
	)
	log.With(zap.String("id", id), zap.String("text", text)).
		Debug("updating document")

	err = s.treeCache.Do(ctx, id, func(obj interface{}) error {
		docTree, ok := obj.(tree.ObjectTree)
		if !ok {
			return fmt.Errorf("can't update acl trees with text")
		}

		docTree.Lock()
		defer docTree.Unlock()
		err = s.treeCache.Do(ctx, docTree.Header().AclListId, func(obj interface{}) error {
			aclTree := obj.(list.ACLList)
			aclTree.RLock()
			defer aclTree.RUnlock()

			content := createAppendTextChange(text)
			signable := tree.SignableChangeContent{
				Proto:      content,
				Key:        s.account.Account().SignKey,
				Identity:   s.account.Account().Identity,
				IsSnapshot: false,
			}
			ch, err = docTree.AddContent(ctx, aclTree, signable)
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
		SnapshotPath: snapshotPath,
	}, header, id))
}

func (s *service) CreateDocumentTree(ctx context.Context, aclListId string, text string) (id string, err error) {
	acc := s.account.Account()
	var (
		ch           *aclpb.RawChange
		header       *aclpb.Header
		snapshotPath []string
		heads        []string
	)
	err = s.treeCache.Do(ctx, aclListId, func(obj interface{}) error {
		t := obj.(list.ACLList)
		t.RLock()
		defer t.RUnlock()

		content := createInitialTextChange(text)
		doc, err := tree.CreateNewTreeStorage(acc, t, content, s.storage.CreateTreeStorage)
		if err != nil {
			return err
		}

		id, err = doc.ID()
		if err != nil {
			return err
		}

		header, err = doc.Header()
		if err != nil {
			return err
		}

		heads = []string{header.FirstId}
		snapshotPath = []string{header.FirstId}
		ch, err = doc.GetRawChange(ctx, header.FirstId)
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
		SnapshotPath: snapshotPath,
	}, header, id))
	if err != nil {
		return "", err
	}
	return id, err
}

func createInitialTextChange(text string) proto.Marshaler {
	return &testchanges.PlainTextChangeData{
		Content: []*testchanges.PlainTextChangeContent{
			createAppendTextChangeContent(text),
		},
		Snapshot: &testchanges.PlainTextChangeSnapshot{Text: text},
	}
}

func createAppendTextChange(text string) proto.Marshaler {
	return &testchanges.PlainTextChangeData{
		Content: []*testchanges.PlainTextChangeContent{
			createAppendTextChangeContent(text),
		},
	}
}

func createAppendTextChangeContent(text string) *testchanges.PlainTextChangeContent {
	return &testchanges.PlainTextChangeContent{
		Value: &testchanges.PlainTextChangeContentValueOfTextAppend{
			TextAppend: &testchanges.PlainTextChangeTextAppend{
				Text: text,
			},
		},
	}
}
