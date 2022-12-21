package filepogreb

import (
	"context"
	"github.com/akrylysov/pogreb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/config"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"go.uber.org/zap"
)

const CName = fileblockstore.CName

var log = logger.NewNamed(CName)

func New() Store {
	return &store{}
}

type Store interface {
	app.ComponentRunnable
	fileblockstore.BlockStore
}

type configSource interface {
	GetFileStorePogreb() config.FileStorePogreb
}

type store struct {
	db   *pogreb.DB
	conf config.FileStorePogreb
}

func (s *store) Init(a *app.App) (err error) {
	s.conf = a.MustComponent("config").(configSource).GetFileStorePogreb()
	return
}

func (s *store) Name() (name string) {
	return CName
}

func (s *store) Run(ctx context.Context) (err error) {
	if s.db, err = pogreb.Open(s.conf.Path, &pogreb.Options{}); err != nil {
		return
	}
	return
}

func (s *store) Get(ctx context.Context, k cid.Cid) (blocks.Block, error) {
	val, err := s.db.Get(k.Bytes())
	if err != nil {
		return nil, fileblockstore.ErrCIDNotFound
	}
	return blocks.NewBlock(val), nil
}

func (s *store) GetMany(ctx context.Context, ks []cid.Cid) <-chan blocks.Block {
	var res = make(chan blocks.Block)
	go func() {
		defer close(res)
		for _, k := range ks {
			b, err := s.Get(ctx, k)
			if err != nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case res <- b:
			}
		}
	}()
	return res
}

func (s *store) Add(ctx context.Context, bs []blocks.Block) error {
	for _, b := range bs {
		log.Debug("put cid", zap.String("cid", b.Cid().String()))
		if err := s.db.Put(b.Cid().Bytes(), b.RawData()); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) Delete(ctx context.Context, c cid.Cid) error {
	return s.db.Delete(c.Bytes())
}

func (s *store) Close(ctx context.Context) (err error) {
	if s.db != nil {
		return s.db.Close()
	}
	return
}
