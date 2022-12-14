package fileservice

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
)

const CName = "common.commonfile.fileservice"

func New() FileService {
	return &fileService{}
}

type FileService interface {
	NewProvider(store fileblockstore.BlockStore) (Provider, error)
	app.Component
}

type fileService struct{}

func (fs *fileService) Init(a *app.App) (err error) {
	return
}

func (fs *fileService) Name() string {
	return CName
}

func (fs *fileService) NewProvider(store fileblockstore.BlockStore) (Provider, error) {
	return newProvider(newBlockService(store))
}
