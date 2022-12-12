package fileservice

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/ipfsstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
)

type blockService struct {
	store ipfsstore.IPFSStore
}

func (bs *blockService) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return bs.store.Get(ctx, c)
}

func (bs *blockService) GetBlocks(ctx context.Context, ks []cid.Cid) <-chan blocks.Block {
	return bs.store.GetMany(ctx, ks)
}

func (bs *blockService) Blockstore() blockstore.Blockstore {
	return nil
}

func (bs *blockService) Exchange() exchange.Interface {
	return nil
}

func (bs *blockService) AddBlock(ctx context.Context, b blocks.Block) error {
	return bs.store.Add(ctx, []blocks.Block{b})
}

func (bs *blockService) AddBlocks(ctx context.Context, b []blocks.Block) error {
	return bs.store.Add(ctx, b)
}

func (bs *blockService) DeleteBlock(ctx context.Context, c cid.Cid) error {
	return bs.store.Delete(ctx, c)
}

func (bs *blockService) Close() error {
	return bs.store.Close()
}
