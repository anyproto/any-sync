package api

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/clientspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/document"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/symmetric"
	"golang.org/x/exp/rand"
)

type Controller interface {
	// DeriveSpace derives the space from current account
	DeriveSpace() (id string, err error)
	// CreateSpace creates new space with random data
	CreateSpace() (id string, err error)
	// AllSpaceIds returns ids of all spaces
	AllSpaceIds() (ids []string, err error)
	// LoadSpace asks node to load a particular space
	LoadSpace(id string) (err error)

	// CreateDocument creates new document in space
	CreateDocument(spaceId string) (id string, err error)
	// AllDocumentIds gets all ids of documents in space
	AllDocumentIds(spaceId string) (ids []string, err error)
	// AddText adds text to space document
	AddText(spaceId, documentId, text string) (err error)
	// DumpDocumentTree dumps the tree data into string
	DumpDocumentTree(spaceId, documentId string) (dump string, err error)

	ValidInvites(spaceId string) (invites []string, err error)
	GenerateInvite(spaceId string) (invite string, err error)
	JoinSpace(invite string) (err error)
}

type controller struct {
	spaceService   clientspace.Service
	storageService storage.ClientStorage
	docService     document.Service
	account        account.Service
}

func newController(spaceService clientspace.Service,
	storageService storage.ClientStorage,
	docService document.Service,
	account account.Service) Controller {
	return &controller{
		spaceService:   spaceService,
		storageService: storageService,
		docService:     docService,
		account:        account,
	}
}

func (c *controller) DeriveSpace() (id string, err error) {
	sp, err := c.spaceService.DeriveSpace(context.Background(), commonspace.SpaceDerivePayload{
		SigningKey:    c.account.Account().SignKey,
		EncryptionKey: c.account.Account().EncKey,
	})
	if err != nil {
		return
	}
	id = sp.Id()
	return
}

func (c *controller) CreateSpace() (id string, err error) {
	key, err := symmetric.NewRandom()
	if err != nil {
		return
	}
	sp, err := c.spaceService.CreateSpace(context.Background(), commonspace.SpaceCreatePayload{
		SigningKey:     c.account.Account().SignKey,
		EncryptionKey:  c.account.Account().EncKey,
		ReadKey:        key.Bytes(),
		ReplicationKey: rand.Uint64(),
	})
	if err != nil {
		return
	}
	id = sp.Id()
	return
}

func (c *controller) AllSpaceIds() (ids []string, err error) {
	return c.storageService.AllSpaceIds()
}

func (c *controller) LoadSpace(id string) (err error) {
	//TODO implement me
	panic("implement me")
}

func (c *controller) CreateDocument(spaceId string) (id string, err error) {
	return c.docService.CreateDocument(spaceId)
}

func (c *controller) AllDocumentIds(spaceId string) (ids []string, err error) {
	return c.docService.AllDocumentIds(spaceId)
}

func (c *controller) AddText(spaceId, documentId, text string) (err error) {
	return c.docService.AddText(spaceId, documentId, text)
}

func (c *controller) DumpDocumentTree(spaceId, documentId string) (dump string, err error) {
	return c.docService.DumpDocumentTree(spaceId, documentId)
}

func (c *controller) ValidInvites(spaceId string) (invites []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (c *controller) GenerateInvite(spaceId string) (invite string, err error) {
	//TODO implement me
	panic("implement me")
}

func (c *controller) JoinSpace(invite string) (err error) {
	//TODO implement me
	panic("implement me")
}
