package fileservice

import (
	"context"
	"io"

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
)

// FileHandler contains all the core file operations logic
// It can be used directly with any BlockStore implementation
type FileHandler struct {
	bs        fileblockstore.BlockStore
	dagService ipld.DAGService
	prefix    cid.Prefix
}

// NewFileHandler creates a new FileHandler with the given BlockStore
func NewFileHandler(bs fileblockstore.BlockStore) *FileHandler {
	// Initialize CID prefix for version 1
	v1CidPrefix := cid.Prefix{
		Version:  1,
		Codec:    cid.DagProtobuf,
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}

	return &FileHandler{
		bs:         bs,
		dagService: merkledag.NewDAGService(newBlockService(bs)),
		prefix:     v1CidPrefix,
	}
}

// AddFile adds file to ipfs storage
func (fh *FileHandler) AddFile(ctx context.Context, r io.Reader) (ipld.Node, error) {
	dbp := helpers.DagBuilderParams{
		Dagserv:    fh.dagService,
		Maxlinks:   helpers.DefaultLinksPerBlock,
		CidBuilder: &fh.prefix,
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

// GetFile gets file from ipfs storage
func (fh *FileHandler) GetFile(ctx context.Context, c cid.Cid) (ufsio.ReadSeekCloser, error) {
	log.Debug("get file", zap.String("cid", c.String()))
	n, err := fh.dagService.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return ufsio.NewDagReader(ctx, n, fh.dagService)
}

// HasCid checks if CID exists
func (fh *FileHandler) HasCid(ctx context.Context, c cid.Cid) (exists bool, err error) {
	if localBS, ok := fh.bs.(fileblockstore.BlockStoreLocal); ok {
		res, err := localBS.ExistsCids(ctx, []cid.Cid{c})
		if err != nil {
			return false, err
		}
		return len(res) > 0, nil
	}
	// Fallback for non-local blockstores
	_, err = fh.bs.Get(ctx, c)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// DAGService returns the underlying DAG service
func (fh *FileHandler) DAGService() ipld.DAGService {
	return fh.dagService
}