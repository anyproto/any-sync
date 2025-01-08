package migration

import (
	"context"
	"errors"
	"fmt"

	anystore "github.com/anyproto/any-store"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/spacestorage/oldstorage"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/slice"
)

type treeMigrator struct {
	idStack    []string
	cache      map[string]*objecttree.Change
	storage    oldstorage.TreeStorage
	builder    objecttree.ChangeBuilder
	allChanges []*treechangeproto.RawTreeChangeWithId

	keyStorage crypto.KeyStorage
	aclList    list.AclList
}

func newTreeMigrator(keyStorage crypto.KeyStorage, aclList list.AclList) *treeMigrator {
	return &treeMigrator{
		keyStorage: keyStorage,
		aclList:    aclList,
	}
}

func (tm *treeMigrator) migrateTreeStorage(ctx context.Context, storage oldstorage.TreeStorage, headStorage headstorage.HeadStorage, store anystore.DB) error {
	rootChange, err := storage.Root()
	if err != nil {
		return err
	}
	tm.allChanges = []*treechangeproto.RawTreeChangeWithId{rootChange}
	tm.storage = storage
	tm.cache = make(map[string]*objecttree.Change)
	tm.builder = objecttree.NewChangeBuilder(tm.keyStorage, rootChange)
	heads, err := storage.Heads()
	if err != nil {
		return fmt.Errorf("migration: failed to get heads: %w", err)
	}
	tm.dfs(ctx, heads, rootChange.Id)
	newStorage, err := objecttree.CreateStorage(ctx, rootChange, headStorage, store)
	if err != nil && !errors.Is(err, treestorage.ErrTreeExists) {
		return fmt.Errorf("migration: failed to create new storage: %w", err)
	}
	if errors.Is(err, treestorage.ErrTreeExists) {
		newStorage, err = objecttree.NewStorage(ctx, rootChange.Id, headStorage, store)
		if err != nil {
			return fmt.Errorf("migration: failed to start old storage: %w", err)
		}
	}
	objTree, err := objecttree.BuildObjectTree(newStorage, tm.aclList)
	if err != nil {
		return fmt.Errorf("migration: failed to build object tree: %w", err)
	}
	addPayload := objecttree.RawChangesPayload{
		NewHeads:     heads,
		RawChanges:   tm.allChanges,
		SnapshotPath: []string{rootChange.Id},
	}
	objTree.Lock()
	res, err := objTree.AddRawChanges(ctx, addPayload)
	objTree.Unlock()
	if err != nil {
		return fmt.Errorf("migration: failed to add raw changes: %w", err)
	}
	if !slice.UnsortedEquals(res.Heads, heads) {
		return fmt.Errorf("migration: heads mismatch: %v != %v", res.Heads, heads)
	}
	return nil
}

func (tm *treeMigrator) dfs(ctx context.Context, heads []string, breakpoint string) {
	tm.idStack = tm.idStack[:0]
	uniqMap := map[string]struct{}{breakpoint: {}}
	tm.idStack = append(tm.idStack, heads...)

	for len(tm.idStack) > 0 {
		id := tm.idStack[len(tm.idStack)-1]
		tm.idStack = tm.idStack[:len(tm.idStack)-1]
		if _, exists := uniqMap[id]; exists {
			continue
		}

		ch, err := tm.loadChange(ctx, id)
		if err != nil {
			continue
		}

		uniqMap[id] = struct{}{}
		for _, prev := range ch.PreviousIds {
			if _, exists := uniqMap[prev]; exists {
				continue
			}
			tm.idStack = append(tm.idStack, prev)
		}
	}
}

func (tm *treeMigrator) loadChange(ctx context.Context, id string) (ch *objecttree.Change, err error) {
	if ch, ok := tm.cache[id]; ok {
		return ch, nil
	}

	change, err := tm.storage.GetRawChange(ctx, id)
	if err != nil {
		return nil, err
	}
	tm.allChanges = append(tm.allChanges, change)
	ch, err = tm.builder.UnmarshallReduced(change)
	if err != nil {
		return nil, err
	}
	tm.cache[id] = ch
	return ch, nil
}
