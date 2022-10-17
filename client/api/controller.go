package api

type Controller interface {
	// DeriveSpace derives the space from current account
	DeriveSpace() (id string, err error)
	// CreateSpace creates new space with random data
	CreateSpace() (id string, err error)
	GetAllSpacesIds() (ids []string, err error)
	// LoadSpace asks node to load a particular space
	LoadSpace(id string) (err error)

	CreateDocument(spaceId string) (id string, err error)
	GetAllDocumentIds(spaceId string) (ids []string, err error)
	AddText(documentId, text string) (err error)
	DumpDocumentTree(documentId string) (err error)

	GetValidInvites(spaceId string) (invites []string, err error)
	GenerateInvite(spaceId string) (invite string, err error)
	JoinSpace(invite string) (err error)
}
