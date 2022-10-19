package document

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace/clientcache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/treegetter"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
	"go.uber.org/zap"
)

type Service interface {
	app.Component
	updatelistener.UpdateListener
	CreateDocument(spaceId string) (id string, err error)
	AllDocumentIds(spaceId string) (ids []string, err error)
	AddText(spaceId, documentId, text string) (err error)
	DumpDocumentTree(spaceId, documentId string) (dump string, err error)
}

const CName = "client.document"

var log = logger.NewNamed(CName)

type service struct {
	account      account.Service
	spaceService clientspace.Service
	cache        clientcache.TreeCache
}

func New() Service {
	return &service{}
}

func (s *service) Init(a *app.App) (err error) {
	s.account = a.MustComponent(account.CName).(account.Service)
	s.spaceService = a.MustComponent(clientspace.CName).(clientspace.Service)
	s.cache = a.MustComponent(treegetter.CName).(clientcache.TreeCache)
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateDocument(spaceId string) (id string, err error) {
	space, err := s.spaceService.GetSpace(context.Background(), spaceId)
	if err != nil {
		return
	}
	doc, err := createTextDocument(context.Background(), space, s.account, s)
	if err != nil {
		return
	}
	id = doc.Tree().ID()
	return
}

func (s *service) AllDocumentIds(spaceId string) (ids []string, err error) {
	space, err := s.spaceService.GetSpace(context.Background(), spaceId)
	if err != nil {
		return
	}
	ids = space.StoredIds()
	return
}

func (s *service) AddText(spaceId, documentId, text string) (err error) {
	doc, err := s.cache.GetDocument(context.Background(), spaceId, documentId)
	if err != nil {
		return
	}
	return doc.AddText(text)
}

func (s *service) DumpDocumentTree(spaceId, documentId string) (dump string, err error) {
	doc, err := s.cache.GetDocument(context.Background(), spaceId, documentId)
	if err != nil {
		return
	}
	return doc.Tree().DebugDump()
}

func (s *service) Update(tree tree.ObjectTree) {
	log.With(
		zap.Strings("heads", tree.Heads()),
		zap.String("tree id", tree.ID())).
		Debug("updating tree")
}

func (s *service) Rebuild(tree tree.ObjectTree) {
	log.With(
		zap.Strings("heads", tree.Heads()),
		zap.String("tree id", tree.ID())).
		Debug("rebuilding tree")
}
