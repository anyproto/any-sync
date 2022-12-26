package fileblockstore

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileproto/fileprotoerr"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

var log = logger.NewNamed(CName)

var (
	ErrCIDNotFound   = fileprotoerr.ErrCIDNotFound
	ErrCIDUnexpected = fileprotoerr.ErrUnexpected
)

const CName = "common.commonfile.fileblockstore"

type ctxKey uint

const (
	ctxKeySpaceId ctxKey = iota
)

type BlockStore interface {
	Get(ctx context.Context, k cid.Cid) (blocks.Block, error)
	GetMany(ctx context.Context, ks []cid.Cid) <-chan blocks.Block
	Add(ctx context.Context, b []blocks.Block) error
	Delete(ctx context.Context, c cid.Cid) error
}

type BlockStoreLocal interface {
	BlockStore
	ExistsCids(ctx context.Context, ks []cid.Cid) (exists []cid.Cid, err error)
	NotExistsBlocks(ctx context.Context, bs []blocks.Block) (notExists []blocks.Block, err error)
}

type BlockStoreSpaceIds interface {
	SpaceIds() []string
}

func CtxWithSpaceId(ctx context.Context, spaceId string) context.Context {
	return context.WithValue(ctx, ctxKeySpaceId, spaceId)
}

func CtxGetSpaceId(ctx context.Context) (spaceId string) {
	spaceId, _ = ctx.Value(ctxKeySpaceId).(string)
	return
}
