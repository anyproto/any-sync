//go:build !js

package fileservice

import (
	"context"
	"io"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonfile/fileblockstore"
	ufsio "github.com/ipfs/boxo/ipld/unixfs/io"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

const CName = "common.commonfile.fileservice"

var log = logger.NewNamed(CName)

const (
	ChunkSize = 1 << 20
)

func New() FileService {
	return &fileService{}
}

type FileService interface {
	// GetFile gets file from ipfs storage
	GetFile(ctx context.Context, c cid.Cid) (ufsio.ReadSeekCloser, error)
	// AddFile adds file to ipfs storage
	AddFile(ctx context.Context, r io.Reader) (ipld.Node, error)
	// DAGService returns ipld.DAGService object
	DAGService() ipld.DAGService
	// HasCid checks is CID exists
	HasCid(ctx context.Context, c cid.Cid) (exists bool, err error)
	app.Component
}

// fileService is a thin wrapper for dependency injection
type fileService struct {
	*FileHandler
}

func (fs *fileService) Init(a *app.App) (err error) {
	bs := a.MustComponent(fileblockstore.CName).(fileblockstore.BlockStoreLocal)
	fs.FileHandler = NewFileHandler(bs)
	return nil
}

func (fs *fileService) Name() string {
	return CName
}