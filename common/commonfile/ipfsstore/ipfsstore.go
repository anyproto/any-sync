package ipfsstore

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

var log = logger.NewNamed("filenode.ipfsstore")

type IPFSStore interface {
	Get(ctx context.Context, k cid.Cid) (blocks.Block, error)
	GetMany(ctx context.Context, ks []cid.Cid) <-chan blocks.Block
	Add(ctx context.Context, b []blocks.Block) error
	Delete(ctx context.Context, c cid.Cid) error
	Close() (err error)
}
