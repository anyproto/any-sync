package ipfs

import (
	"context"
	"fmt"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	chunker "github.com/ipfs/go-ipfs-chunker"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-unixfs/importer/balanced"
	"github.com/ipfs/go-unixfs/importer/helpers"
	ufsio "github.com/ipfs/go-unixfs/io"
	"github.com/multiformats/go-multihash"
	"io"
)

func init() {
	ipld.Register(cid.DagProtobuf, merkledag.DecodeProtobufBlock)
	ipld.Register(cid.Raw, merkledag.DecodeRawBlock)
	ipld.Register(cid.DagCBOR, cbor.DecodeBlock) // need to decode CBOR
}

// NewProvider creates IPFS provider with given blockstore
func NewProvider(bs blockservice.BlockService) Provider {
	return &provider{
		merkledag: merkledag.NewDAGService(bs),
	}
}

// Provider provides high level function for ipfs stack
type Provider interface {
	// GetFile gets file from ipfs storage
	GetFile(ctx context.Context, c cid.Cid) (ufsio.ReadSeekCloser, error)
	// AddFile adds file to ipfs storage
	AddFile(ctx context.Context, r io.Reader) (ipld.Node, error)
}

type provider struct {
	merkledag ipld.DAGService
}

func (p *provider) AddFile(ctx context.Context, r io.Reader) (ipld.Node, error) {
	prefix, err := merkledag.PrefixForCidVersion(1)
	if err != nil {
		return nil, fmt.Errorf("bad CID Version: %s", err)
	}

	hashFunCode, ok := multihash.Names["sha2-256"]
	if !ok {
		return nil, fmt.Errorf("unrecognized hash function")
	}
	prefix.MhType = hashFunCode
	prefix.MhLength = -1

	dbp := helpers.DagBuilderParams{
		Dagserv:    p.merkledag,
		Maxlinks:   helpers.DefaultLinksPerBlock,
		CidBuilder: &prefix,
	}
	dbh, err := dbp.New(chunker.DefaultSplitter(r))
	if err != nil {
		return nil, err
	}
	return balanced.Layout(dbh)
}

func (p *provider) GetFile(ctx context.Context, c cid.Cid) (ufsio.ReadSeekCloser, error) {
	n, err := p.merkledag.Get(ctx, c)
	n.Links()
	if err != nil {
		return nil, err
	}
	return ufsio.NewDagReader(ctx, n, p.merkledag)
}
