package fileservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
)

const CName = "filenode.fileservice"

type FileService interface {
	app.ComponentRunnable
}

type fileService struct {
	blockService *blockService
	dagService   ipld.DAGService
}

func (fs *fileService) Init(a *app.App) (err error) {
	fs.dagService = merkledag.NewDAGService(fs.blockService)
	return
}

func (fs *fileService) Name() string {
	return CName
}

func (fs *fileService) Run(ctx context.Context) (err error) {
	return
}

func (fs *fileService) Close(ctx context.Context) (err error) {
	return
}
