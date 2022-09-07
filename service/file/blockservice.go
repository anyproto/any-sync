package file

import (
	"context"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
)

// client get: local -> remote
// client put: local -> remote
// client delete: local -> remote
// client clear: local
//
// node get: ds
// node put: valid -> ds
// node delete: valid -> ds
type ipfsBlockService struct{}

func (bs *ipfsBlockService) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	//TODO implement me
	panic("implement me")
}

func (bs *ipfsBlockService) GetBlocks(ctx context.Context, ks []cid.Cid) <-chan blocks.Block {
	//TODO implement me
	panic("implement me")
}

func (bs *ipfsBlockService) Blockstore() blockstore.Blockstore {
	return nil
}

func (bs *ipfsBlockService) Exchange() exchange.Interface {
	return nil
}

func (bs *ipfsBlockService) AddBlock(ctx context.Context, o blocks.Block) error {
	//TODO implement me
	panic("implement me")
}

func (bs *ipfsBlockService) AddBlocks(ctx context.Context, blocks []blocks.Block) error {
	//TODO implement me
	panic("implement me")
}

func (bs *ipfsBlockService) DeleteBlock(ctx context.Context, o cid.Cid) error {
	//TODO implement me
	panic("implement me")
}

func (bs *ipfsBlockService) Close() error {
	//TODO implement me
	panic("implement me")
}
