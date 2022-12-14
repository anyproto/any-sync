package filestorage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/badgerprovider"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/filestorage/filebadgerstore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore/cachestore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/rpcstore"
	"io"
)

const CName = fileblockstore.CName

func New() FileStorage {
	return &fileStorage{}
}

type FileStorage interface {
	app.ComponentRunnable
	fileblockstore.BlockStore
}

type fileStorage struct {
	fileblockstore.BlockStore
}

func (f *fileStorage) Init(a *app.App) (err error) {
	db := a.MustComponent(badgerprovider.CName).(badgerprovider.BadgerProvider).Badger()
	bs := filebadgerstore.NewBadgerStorage(db)
	f.BlockStore = &cachestore.CacheStore{
		Cache:  bs,
		Origin: a.MustComponent(rpcstore.CName).(rpcstore.Service).NewStore(),
	}
	return
}

func (f *fileStorage) Name() (name string) {
	return CName
}

func (f *fileStorage) Run(ctx context.Context) (err error) {
	return
}

func (f *fileStorage) Close(ctx context.Context) (err error) {
	return f.BlockStore.(io.Closer).Close()
}
