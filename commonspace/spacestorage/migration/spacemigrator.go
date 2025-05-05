package migration

import (
	"context"
	"errors"
	"fmt"
	"time"

	anystore "github.com/anyproto/any-store"
	"github.com/anyproto/any-store/anyenc"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage/oldstorage"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/slice"
)

const (
	migratedColl    = "migration"
	migratedDoc     = "state"
	migratedTimeKey = "t"
)

var ErrAlreadyMigrated = errors.New("already migrated")

type SpaceMigrator interface {
	MigrateId(ctx context.Context, id string, progress Progress) error
}

type Progress interface {
	AddDone(done int64)
}

type RemoveFunc func(newStorage spacestorage.SpaceStorage, rootPath string) error

type spaceMigrator struct {
	oldProvider oldstorage.SpaceStorageProvider
	newProvider spacestorage.SpaceStorageProvider
	numParallel int
	rootPath    string
	removeFunc  RemoveFunc
}

func NewSpaceMigrator(oldProvider oldstorage.SpaceStorageProvider, newProvider spacestorage.SpaceStorageProvider, numParallel int, rootPath string, removeFunc RemoveFunc) SpaceMigrator {
	return &spaceMigrator{
		oldProvider: oldProvider,
		newProvider: newProvider,
		numParallel: numParallel,
		rootPath:    rootPath,
		removeFunc:  removeFunc,
	}
}

func (s *spaceMigrator) MigrateId(ctx context.Context, id string, progress Progress) error {
	migrated, storage := s.checkMigrated(ctx, id)
	if migrated {
		storage.Close(ctx)
		return ErrAlreadyMigrated
	}
	if !migrated {
		err := s.removeFunc(storage, s.rootPath)
		if err != nil {
			return fmt.Errorf("migration: failed to remove new storage: %w", err)
		}
	}
	oldStorage, err := s.oldProvider.WaitSpaceStorage(ctx, id)
	if err != nil {
		return fmt.Errorf("migration: failed to get old space storage: %w", err)
	}
	defer func() {
		err := oldStorage.Close(ctx)
		if err != nil {
			log.Warn("migration: failed to close old storage", zap.Error(err))
		}
	}()
	header, err := oldStorage.SpaceHeader()
	if err != nil {
		return fmt.Errorf("migration: failed to get space header: %w", err)
	}
	settingsId := oldStorage.SpaceSettingsId()
	settingsRoot, err := oldStorage.TreeRoot(settingsId)
	if err != nil {
		return fmt.Errorf("migration: failed to get space settings root: %w", err)
	}
	aclStorage, err := oldStorage.AclStorage()
	if err != nil {
		return fmt.Errorf("migration: failed to get acl storage: %w", err)
	}
	aclRoot, err := aclStorage.Root()
	if err != nil {
		return fmt.Errorf("migration: failed to get acl root: %w", err)
	}
	createPayload := spacestorage.SpaceStorageCreatePayload{
		AclWithId:           aclRoot,
		SpaceHeaderWithId:   header,
		SpaceSettingsWithId: settingsRoot,
	}
	newStorage, err := s.newProvider.CreateSpaceStorage(ctx, createPayload)
	if err != nil {
		return fmt.Errorf("migration: failed to create new space storage: %w", err)
	}
	defer func() {
		err := newStorage.Close(ctx)
		if err != nil {
			log.Warn("migration: failed to close old storage", zap.Error(err))
		}
	}()
	aclList, err := migrateAclList(ctx, aclStorage, newStorage.HeadStorage(), newStorage.AnyStore())
	if err != nil {
		return fmt.Errorf("migration: failed to migrate acl list: %w", err)
	}
	executor := newMigratePool(ctx, s.numParallel, 0)
	storedIds, err := oldStorage.StoredIds()
	if err != nil {
		return fmt.Errorf("migration: failed to get stored ids: %w", err)
	}
	treeMigrators := make([]*objecttree.TreeMigrator, 0, s.numParallel)
	ch := make(chan *objecttree.TreeMigrator, s.numParallel)
	for i := 0; i < s.numParallel; i++ {
		treeMigrators = append(treeMigrators, objecttree.NewTreeMigrator(crypto.NewKeyStorage(), aclList))
		ch <- treeMigrators[i]
	}
	slices.Sort(storedIds)
	storedIds = slice.DiscardDuplicatesSorted(storedIds)
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
			err = tm.MigrateTreeStorage(ctx, treeStorage, newStorage.HeadStorage(), newStorage.AnyStore())
			if err != nil {
				return
			}
		})
		if err != nil {
			return fmt.Errorf("migration: failed to add task: %w", err)
		}
	}
	executor.Run()
	err = executor.Wait()
	if err != nil {
		return fmt.Errorf("migration: failed to wait for executor: %w", err)
	}
	if err := s.migrateHash(ctx, oldStorage, newStorage); err != nil {
		log.Warn("migration: failed to migrate hash", zap.Error(err))
	}
	return s.setMigrated(ctx, newStorage.AnyStore())
}

func (s *spaceMigrator) migrateHash(ctx context.Context, oldStorage oldstorage.SpaceStorage, newStorage spacestorage.SpaceStorage) error {
	spaceHash, err := oldStorage.ReadSpaceHash()
	if err != nil {
		return err
	}
	return newStorage.StateStorage().SetHash(ctx, spaceHash)
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
