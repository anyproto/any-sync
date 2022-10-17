package document

import "github.com/anytypeio/go-anytype-infrastructure-experiments/app"

type Service interface {
	app.Component
	CreateDocument(spaceId string) (id string, err error)
	GetAllDocumentIds(spaceId string) (ids []string, err error)
	AddText(documentId, text string) (err error)
	DumpDocumentTree(documentId string) (err error)
}

const CName = "client.document"
