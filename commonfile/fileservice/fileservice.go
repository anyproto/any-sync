package fileservice

import (
	"context"
	"fmt"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonfile/fileblockstore"
	chunker "github.com/ipfs/boxo/chunker"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/unixfs/importer/balanced"
	"github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	ufsio "github.com/ipfs/boxo/ipld/unixfs/io"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/multiformats/go-multihash"
	"go.uber.org/zap"
	"io"
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
type fileService struct {
	bs        fileblockstore.BlockStoreLocal
	merkledag ipld.DAGService
	prefix    cid.Prefix
}

func (fs *fileService) Init(a *app.App) (err error) {
	prefix, err := merkledag.PrefixForCidVersion(1)
	if err != nil {
		return fmt.Errorf("bad CID Version: %s", err)
	}
	hashFunCode, ok := multihash.Names["sha2-256"]
	if !ok {
		return fmt.Errorf("unrecognized hash function")
	}
	prefix.MhType = hashFunCode
	prefix.MhLength = -1
	fs.bs = a.MustComponent(fileblockstore.CName).(fileblockstore.BlockStoreLocal)
	fs.merkledag = merkledag.NewDAGService(newBlockService(fs.bs))
	fs.prefix = prefix
	return
}

func (fs *fileService) Name() string {
	return CName
}

func (fs *fileService) DAGService() ipld.DAGService {
	return fs.merkledag
}

func (fs *fileService) AddFile(ctx context.Context, r io.Reader) (ipld.Node, error) {
	dbp := helpers.DagBuilderParams{
		Dagserv:    fs.merkledag,
		Maxlinks:   helpers.DefaultLinksPerBlock,
		CidBuilder: &fs.prefix,
	}
	dbh, err := dbp.New(chunker.NewSizeSplitter(r, ChunkSize))
	if err != nil {
		return nil, err
	}
	n, err := balanced.Layout(dbh)
	if err != nil {
		return nil, err
	}
	log.Debug("add file", zap.String("cid", n.Cid().String()))
	return n, nil
}

func (fs *fileService) GetFile(ctx context.Context, c cid.Cid) (ufsio.ReadSeekCloser, error) {
	log.Debug("get file", zap.String("cid", c.String()))
	n, err := fs.merkledag.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return ufsio.NewDagReader(ctx, n, fs.merkledag)
}

func (fs *fileService) HasCid(ctx context.Context, c cid.Cid) (exists bool, err error) {
	res, err := fs.bs.ExistsCids(ctx, []cid.Cid{c})
	if err != nil {
		return
	}
	return len(res) > 0, nil
}
