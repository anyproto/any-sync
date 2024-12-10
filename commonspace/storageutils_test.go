package commonspace

import (
	"context"
	"os"
	"path"

	anystore "github.com/anyproto/any-store"
	"golang.org/x/sys/unix"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
)

type spaceStorageProvider struct {
	rootPath string
}

func (s *spaceStorageProvider) Run(ctx context.Context) (err error) {
	return nil
}

func (s *spaceStorageProvider) Close(ctx context.Context) (err error) {
	return unix.Rmdir(s.rootPath)
}

func (s *spaceStorageProvider) Init(a *app.App) (err error) {
	return nil
}

func (s *spaceStorageProvider) Name() (name string) {
	return spacestorage.CName
}

func (s *spaceStorageProvider) WaitSpaceStorage(ctx context.Context, id string) (spacestorage.SpaceStorage, error) {
	dbPath := path.Join(s.rootPath, id)
	if _, err := os.Stat(dbPath); err != nil {
		return nil, err
	}
	db, err := anystore.Open(ctx, dbPath, nil)
	if err != nil {
		return nil, err
	}
	return spacestorage.New(ctx, id, db)
}

func (s *spaceStorageProvider) SpaceExists(id string) bool {
	if id == "" {
		return false
	}
	dbPath := path.Join(s.rootPath, id)
	if _, err := os.Stat(dbPath); err != nil {
		return false
	}
	return true
}

func (s *spaceStorageProvider) CreateSpaceStorage(ctx context.Context, payload spacestorage.SpaceStorageCreatePayload) (spacestorage.SpaceStorage, error) {
	id := payload.SpaceHeaderWithId.Id
	if s.SpaceExists(id) {
		return nil, spacestorage.ErrSpaceStorageExists
	}
	dbPath := path.Join(s.rootPath, id)
	db, err := anystore.Open(ctx, dbPath, nil)
	if err != nil {
		return nil, err
	}
	return spacestorage.Create(ctx, db, payload)
}
