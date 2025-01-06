package migration

import (
	"context"
	"errors"
	"fmt"

	anystore "github.com/anyproto/any-store"

	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage/oldstorage"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/util/crypto"
)

type spaceMigrator struct {
	oldStorage  oldstorage.SpaceStorage
	numParallel int
}

func newSpaceMigrator(spaceStorage oldstorage.SpaceStorage, numParallel int) *spaceMigrator {
	return &spaceMigrator{
		oldStorage:  spaceStorage,
		numParallel: numParallel,
	}
}

func (s *spaceMigrator) migrateSpaceStorage(ctx context.Context, store anystore.DB) (spacestorage.SpaceStorage, error) {
	header, err := s.oldStorage.SpaceHeader()
	if err != nil {
		return nil, fmt.Errorf("migration: failed to get space header: %w", err)
	}
	settingsId := s.oldStorage.SpaceSettingsId()
	settingsRoot, err := s.oldStorage.TreeRoot(settingsId)
	if err != nil {
		return nil, fmt.Errorf("migration: failed to get space settings root: %w", err)
	}
	aclStorage, err := s.oldStorage.AclStorage()
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
	newStorage, err := spacestorage.Create(ctx, store, createPayload)
	if err != nil {
		return nil, fmt.Errorf("migration: failed to create new space storage: %w", err)
	}
	aclList, err := migrateAclList(ctx, aclStorage, newStorage.HeadStorage(), store)
	if err != nil {
		return nil, fmt.Errorf("migration: failed to migrate acl list: %w", err)
	}
	executor := streampool.NewExecPool(s.numParallel, 0)
	storedIds, err := s.oldStorage.StoredIds()
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
				ch <- tm
			}()
			treeStorage, err := s.oldStorage.TreeStorage(id)
			if err != nil {
				allErrors = append(allErrors, fmt.Errorf("migration: failed to get tree storage: %w", err))
				return
			}
			err = tm.migrateTreeStorage(ctx, treeStorage, newStorage.HeadStorage(), store)
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
