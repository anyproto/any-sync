package migration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage/oldstorage"
	"github.com/anyproto/any-sync/util/crypto"
)

const (
	migratedColl    = "migration"
	migratedDoc     = "state"
	migratedTimeKey = "t"
)

var ErrAlreadyMigrated = errors.New("already migrated")

type SpaceMigrator interface {
	MigrateId(ctx context.Context, id string, progress Progress) (spacestorage.SpaceStorage, error)
	CheckMigrated(ctx context.Context, id string) (bool, error)
}

type Progress interface {
	AddDone(done int64)
}

type spaceMigrator struct {
	oldProvider oldstorage.SpaceStorageProvider
	newProvider spacestorage.SpaceStorageProvider
	numParallel int
	rootPath    string
}

func NewSpaceMigrator(oldProvider oldstorage.SpaceStorageProvider, newProvider spacestorage.SpaceStorageProvider, numParallel int, rootPath string) SpaceMigrator {
	return &spaceMigrator{
		oldProvider: oldProvider,
		newProvider: newProvider,
		numParallel: numParallel,
		rootPath:    rootPath,
	}
}

func (s *spaceMigrator) CheckMigrated(ctx context.Context, id string) (bool, error) {
	migrated, storage := s.checkMigrated(ctx, id)
	if storage != nil {
		return migrated, storage.Close(ctx)
	}
	return false, nil
}

func (s *spaceMigrator) MigrateId(ctx context.Context, id string, progress Progress) (spacestorage.SpaceStorage, error) {
	migrated, storage := s.checkMigrated(ctx, id)
	if migrated {
		storage.Close(ctx)
		return nil, ErrAlreadyMigrated
	}
	if storage != nil {
		err := storage.Close(ctx)
		if err != nil {
			return nil, fmt.Errorf("migration: failed to close old storage: %w", err)
		}
		os.RemoveAll(filepath.Join(s.rootPath, id))
	}
	oldStorage, err := s.oldProvider.WaitSpaceStorage(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("migration: failed to get old space storage: %w", err)
	}
	header, err := oldStorage.SpaceHeader()
	if err != nil {
		return nil, fmt.Errorf("migration: failed to get space header: %w", err)
	}
	settingsId := oldStorage.SpaceSettingsId()
	settingsRoot, err := oldStorage.TreeRoot(settingsId)
	if err != nil {
		return nil, fmt.Errorf("migration: failed to get space settings root: %w", err)
	}
	aclStorage, err := oldStorage.AclStorage()
	if err != nil {
		return nil, fmt.Errorf("migration: failed to get acl storage: %w", err)
	}
	aclRoot, err := aclStorage.Root()
	if err != nil {
		return nil, fmt.Errorf("migration: failed to get acl root: %w", err)
	}
	createPayload := spacestorage.SpaceStorageCreatePayload{
		AclWithId:           aclRoot,
		SpaceHeaderWithId:   header,
		SpaceSettingsWithId: settingsRoot,
	}
	newStorage, err := s.newProvider.CreateSpaceStorage(ctx, createPayload)
	if err != nil {
		return nil, fmt.Errorf("migration: failed to create new space storage: %w", err)
	}
	aclList, err := migrateAclList(ctx, aclStorage, newStorage.HeadStorage(), newStorage.AnyStore())
	if err != nil {
		return nil, fmt.Errorf("migration: failed to migrate acl list: %w", err)
	}
	executor := newMigratePool(ctx, s.numParallel, 0)
	storedIds, err := oldStorage.StoredIds()
	if err != nil {
		return nil, fmt.Errorf("migration: failed to get stored ids: %w", err)
	}
	treeMigrators := make([]*treeMigrator, 0, s.numParallel)
	ch := make(chan *treeMigrator, s.numParallel)
	for i := 0; i < s.numParallel; i++ {
		treeMigrators = append(treeMigrators, newTreeMigrator(crypto.NewKeyStorage(), aclList))
		ch <- treeMigrators[i]
	}
	var allErrors []error
	for _, id := range storedIds {
		err := executor.Add(ctx, func() {
			tm := <-ch
			defer func() {
				progress.AddDone(1)
				ch <- tm
			}()
			treeStorage, err := oldStorage.TreeStorage(id)
			if err != nil {
				log.Warn("migration: failed to get old tree storage", zap.String("id", id), zap.Error(err))
				return
			}
			err = tm.migrateTreeStorage(ctx, treeStorage, newStorage.HeadStorage(), newStorage.AnyStore())
			if err != nil {
				allErrors = append(allErrors, fmt.Errorf("migration: failed to migrate tree storage: %w", err))
				return
			}
		})
		if err != nil {
			return nil, fmt.Errorf("migration: failed to add task: %w", err)
		}
	}
	executor.Run()
	err = executor.Wait()
	if err != nil {
		return nil, fmt.Errorf("migration: failed to wait for executor: %w", err)
	}
	if len(allErrors) > 0 {
		return nil, fmt.Errorf("migration failed: %w", errors.Join(allErrors...))
	}
	return newStorage, s.setMigrated(ctx, newStorage.AnyStore())
}

func (s *spaceMigrator) checkMigrated(ctx context.Context, id string) (bool, spacestorage.SpaceStorage) {
	storage, err := s.newProvider.WaitSpaceStorage(ctx, id)
	if err != nil {
		return false, nil
	}
	coll, err := storage.AnyStore().OpenCollection(ctx, migratedColl)
	if err != nil {
		return false, storage
	}
	_, err = coll.FindId(ctx, migratedDoc)
	if err != nil {
		return false, storage
	}
	return true, storage
}

func (s *spaceMigrator) setMigrated(ctx context.Context, anyStore anystore.DB) error {
	coll, err := anyStore.Collection(ctx, migratedColl)
	if err != nil {
		return fmt.Errorf("migration: failed to get collection: %w", err)
	}
	arena := &anyenc.Arena{}
	tx, err := coll.WriteTx(ctx)
	if err != nil {
		return err
	}
	newVal := arena.NewObject()
	newVal.Set(migratedTimeKey, arena.NewNumberFloat64(float64(time.Now().Unix())))
	newVal.Set("id", arena.NewString(migratedDoc))
	err = coll.Insert(tx.Context(), newVal)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit()
	if err != nil {
		return nil
	}
	return anyStore.Checkpoint(ctx, true)
}
