package migration

import (
	"context"
	"errors"
	"fmt"

	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage/oldstorage"
	"github.com/anyproto/any-sync/util/crypto"
)

type SpaceMigrator interface {
	MigrateId(ctx context.Context, id string, progress Progress) (spacestorage.SpaceStorage, error)
}

type Progress interface {
	AddDone(done int64)
}

type spaceMigrator struct {
	oldProvider oldstorage.SpaceStorageProvider
	newProvider spacestorage.SpaceStorageProvider
	numParallel int
}

func NewSpaceMigrator(oldProvider oldstorage.SpaceStorageProvider, newProvider spacestorage.SpaceStorageProvider, numParallel int) SpaceMigrator {
	return &spaceMigrator{
		oldProvider: oldProvider,
		newProvider: newProvider,
		numParallel: numParallel,
	}
}

func (s *spaceMigrator) MigrateId(ctx context.Context, id string, progress Progress) (spacestorage.SpaceStorage, error) {
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
	executor := newMigratePool(s.numParallel, 0)
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
				allErrors = append(allErrors, fmt.Errorf("migration: failed to get tree storage: %w", err))
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
	executor.Close()
	if len(allErrors) > 0 {
		return nil, fmt.Errorf("migration failed: %w", errors.Join(allErrors...))
	}
	return newStorage, nil
}
