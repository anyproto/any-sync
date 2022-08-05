package document

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/acltree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/testchanges/testchangepb"
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
	messageService message.Service
	treeCache      treecache.Service
	account        account.Service
	// to create new documents we need to know all nodes
	nodes []*node.Node
}

type Service interface {
	UpdateDocument(ctx context.Context, id, text string) error
	CreateDocument(ctx context.Context, text string) (string, error)
}

func New() app.Component {
	return &service{}
}

func (s *service) Init(ctx context.Context, a *app.App) (err error) {
	s.account = a.MustComponent(account.CName).(account.Service)
	s.messageService = a.MustComponent(message.CName).(message.Service)
	s.treeCache = a.MustComponent(treecache.CName).(treecache.Service)

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

func (s *service) UpdateDocument(ctx context.Context, id, text string) (err error) {
	var (
		ch           *aclpb.RawChange
		header       *treepb.TreeHeader
		snapshotPath []string
		heads        []string
	)
	log.With(zap.String("id", id), zap.String("text", text)).
		Debug("updating document")

	err = s.treeCache.Do(ctx, id, func(tree acltree.ACLTree) error {
		ch, err = tree.AddContent(ctx, func(builder acltree.ChangeBuilder) error {
			builder.AddChangeContent(
				&testchangepb.PlainTextChangeData{
					Content: []*testchangepb.PlainTextChangeContent{
						createAppendTextChangeContent(text),
					},
				})
			return nil
		})
		if err != nil {
			return err
		}

		id = tree.ID()
		heads = tree.Heads()
		header = tree.Header()
		snapshotPath = tree.SnapshotPath()
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

	return s.messageService.SendToSpace(ctx, "", syncproto.WrapHeadUpdate(&syncproto.SyncHeadUpdate{
		Heads:        heads,
		Changes:      []*aclpb.RawChange{ch},
		TreeId:       id,
		SnapshotPath: snapshotPath,
		TreeHeader:   header,
	}))
}

func (s *service) CreateDocument(ctx context.Context, text string) (id string, err error) {
	acc := s.account.Account()
	var (
		ch           *aclpb.RawChange
		header       *treepb.TreeHeader
		snapshotPath []string
		heads        []string
	)
	log.With(zap.String("id", id), zap.String("text", text)).
		Debug("creating document")

	err = s.treeCache.Create(ctx, func(builder acltree.ChangeBuilder) error {
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

		builder.AddChangeContent(createInitialChangeContent(text))
		return nil
	}, func(tree acltree.ACLTree) error {
		id = tree.ID()
		heads = tree.Heads()
		header = tree.Header()
		snapshotPath = tree.SnapshotPath()
		ch, err = tree.Storage().GetChange(ctx, heads[0])
		if err != nil {
			return err
		}
		log.With(
			zap.String("id", id),
			zap.Strings("heads", heads),
			zap.String("header", header.String())).
			Debug("document created in the database")
		return nil
	})
	if err != nil {
		return "", err
	}

	err = s.messageService.SendToSpace(ctx, "", syncproto.WrapHeadUpdate(&syncproto.SyncHeadUpdate{
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

func createInitialChangeContent(text string) proto.Marshaler {
	return &testchangepb.PlainTextChangeData{
		Content: []*testchangepb.PlainTextChangeContent{
			createAppendTextChangeContent(text),
		},
		Snapshot: &testchangepb.PlainTextChangeSnapshot{Text: text},
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
