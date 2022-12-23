package filestorage

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/badgerprovider"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/filestorage/badgerfilestore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/filestorage/rpcstore"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonfile/fileblockstore"
	"io"
)

const CName = fileblockstore.CName

var log = logger.NewNamed(CName)

func New() FileStorage {
	return &fileStorage{}
}

type FileStorage interface {
	app.ComponentRunnable
	fileblockstore.BlockStore
}

type fileStorage struct {
	fileblockstore.BlockStore
	syncer       *syncer
	syncerCancel context.CancelFunc
}

func (f *fileStorage) Init(a *app.App) (err error) {
	db := a.MustComponent(badgerprovider.CName).(badgerprovider.BadgerProvider).Badger()
	bs := badgerfilestore.NewBadgerStorage(db)
	ps := &proxyStore{
		cache:  bs,
		origin: a.MustComponent(rpcstore.CName).(rpcstore.Service).NewStore(),
		index:  badgerfilestore.NewFileBadgerIndex(db),
	}
	f.BlockStore = ps
	f.syncer = &syncer{ps: ps, done: make(chan struct{})}
	return
}

func (f *fileStorage) Name() (name string) {
	return CName
}

func (f *fileStorage) Run(ctx context.Context) (err error) {
	ctx, f.syncerCancel = context.WithCancel(ctx)
	go f.syncer.run(ctx)
	return
}

func (f *fileStorage) Close(ctx context.Context) (err error) {
	if f.syncerCancel != nil {
		f.syncerCancel()
		<-f.syncer.done
	}
	return f.BlockStore.(io.Closer).Close()
}
