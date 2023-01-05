package fileservice

import (
	"context"
	"fmt"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonfile/fileblockstore"
	"github.com/ipfs/go-cid"
	chunker "github.com/ipfs/go-ipfs-chunker"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-unixfs/importer/balanced"
	"github.com/ipfs/go-unixfs/importer/helpers"
	ufsio "github.com/ipfs/go-unixfs/io"
	"github.com/multiformats/go-multihash"
	"go.uber.org/zap"
	"io"
)

const CName = "common.commonfile.fileservice"

var log = logger.NewNamed(CName)

func New() FileService {
	return &fileService{}
}

type FileService interface {
	// GetFile gets file from ipfs storage
	GetFile(ctx context.Context, c cid.Cid) (ufsio.ReadSeekCloser, error)
	// AddFile adds file to ipfs storage
	AddFile(ctx context.Context, r io.Reader) (ipld.Node, error)
	app.Component
}

type fileService struct {
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
	bs := newBlockService(a.MustComponent(fileblockstore.CName).(fileblockstore.BlockStore))
	fs.merkledag = merkledag.NewDAGService(bs)
	fs.prefix = prefix
	return
}

func (fs *fileService) Name() string {
	return CName
}

func (fs *fileService) AddFile(ctx context.Context, r io.Reader) (ipld.Node, error) {
	dbp := helpers.DagBuilderParams{
		Dagserv:    fs.merkledag,
		Maxlinks:   helpers.DefaultLinksPerBlock,
		CidBuilder: &fs.prefix,
	}
	dbh, err := dbp.New(chunker.DefaultSplitter(r))
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
