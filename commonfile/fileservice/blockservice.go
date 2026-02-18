//go:build !js

package fileservice

import (
	"context"
	"github.com/anyproto/any-sync/commonfile/fileblockstore"
	"github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"
	exchange "github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

func newBlockService(store fileblockstore.BlockStore) blockservice.BlockService {
	return &blockService{store: store}
}

type blockService struct {
	store fileblockstore.BlockStore
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
	return nil
}
