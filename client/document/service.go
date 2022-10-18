package document

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/synctree/updatelistener"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/tree"
)

type Service interface {
	app.Component
	updatelistener.UpdateListener
	CreateDocument(spaceId string) (id string, err error)
	GetAllDocumentIds(spaceId string) (ids []string, err error)
	AddText(spaceId, documentId, text string) (err error)
	DumpDocumentTree(spaceId, documentId string) (dump string, err error)
}

const CName = "client.document"

var log = logger.NewNamed(CName)

type service struct {
	account      account.Service
	spaceService clientspace.Service
	cache        cache.TreeCache
}

func New() Service {
	return &service{}
}

func (s *service) Init(a *app.App) (err error) {
	s.account = a.MustComponent(account.CName).(account.Service)
	s.spaceService = a.MustComponent(clientspace.CName).(clientspace.Service)
	s.cache = a.MustComponent(cache.CName).(cache.TreeCache)
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateDocument(spaceId string) (id string, err error) {
	spaceRef, err := s.spaceService.GetSpace(context.Background(), spaceId)
	if err != nil {
		return
	}
	defer spaceRef.Release()
	doc, err := createTextDocument(context.Background(), spaceRef.Object, s.account, s)
	if err != nil {
		return
	}
	id = doc.Tree().ID()
	return
}

func (s *service) GetAllDocumentIds(spaceId string) (ids []string, err error) {
	spaceRef, err := s.spaceService.GetSpace(context.Background(), spaceId)
	if err != nil {
		return
	}
	defer spaceRef.Release()
	ids = spaceRef.Object.StoredIds()
	return
}

func (s *service) AddText(spaceId, documentId, text string) (err error) {
	doc, err := s.cache.GetTree(context.Background(), spaceId, documentId)
	if err != nil {
		return
	}
	defer doc.Release()
	return doc.TreeContainer.(TextDocument).AddText(text)
}

func (s *service) DumpDocumentTree(spaceId, documentId string) (dump string, err error) {
	doc, err := s.cache.GetTree(context.Background(), spaceId, documentId)
	if err != nil {
		return
	}
	defer doc.Release()
	return doc.TreeContainer.Tree().DebugDump()
}

func (s *service) Update(tree tree.ObjectTree) {

}

func (s *service) Rebuild(tree tree.ObjectTree) {
	//TODO implement me
	panic("implement me")
}
