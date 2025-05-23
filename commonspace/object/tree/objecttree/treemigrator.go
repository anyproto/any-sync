package objecttree

import (
	"context"
	"errors"
	"fmt"

	anystore "github.com/anyproto/any-store"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/anyproto/any-sync/util/slice"
)

type treeStorage interface {
	Id() string
	Root() (*treechangeproto.RawTreeChangeWithId, error)
	Heads() ([]string, error)
	SetHeads(heads []string) error
	AddRawChange(change *treechangeproto.RawTreeChangeWithId) error
	AddRawChangesSetHeads(changes []*treechangeproto.RawTreeChangeWithId, heads []string) error
	GetAllChangeIds() ([]string, error)

	GetRawChange(ctx context.Context, id string) (*treechangeproto.RawTreeChangeWithId, error)
	GetAppendRawChange(ctx context.Context, buf []byte, id string) (*treechangeproto.RawTreeChangeWithId, error)
	HasChange(ctx context.Context, id string) (bool, error)
	Delete() error
}

type changesIterator interface {
	GetAllChanges() ([]*treechangeproto.RawTreeChangeWithId, error)
	IterateChanges(proc func(id string, rawChange []byte) error) error
}

type TreeMigrator struct {
	idStack    []string
	cache      map[string]*Change
	storage    treeStorage
	builder    ChangeBuilder
	allChanges []*treechangeproto.RawTreeChangeWithId

	keyStorage crypto.KeyStorage
	aclList    list.AclList
}

func NewTreeMigrator(keyStorage crypto.KeyStorage, aclList list.AclList) *TreeMigrator {
	return &TreeMigrator{
		keyStorage: keyStorage,
		aclList:    aclList,
	}
}

func (tm *TreeMigrator) MigrateTreeStorage(ctx context.Context, storage treeStorage, headStorage headstorage.HeadStorage, store anystore.DB) error {
	rootChange, err := storage.Root()
	if err != nil {
		return err
	}
	tm.allChanges = []*treechangeproto.RawTreeChangeWithId{rootChange}
	tm.storage = storage
	tm.cache = make(map[string]*Change)
	tm.builder = NewChangeBuilder(tm.keyStorage, rootChange)
	tm.builder = &nonVerifiableChangeBuilder{tm.builder}
	heads, err := storage.Heads()
	if err != nil {
		return fmt.Errorf("migration: failed to get heads: %w", err)
	}
	if iterStore, ok := storage.(changesIterator); ok {
		tm.allChanges, err = iterStore.GetAllChanges()
		if err != nil {
			return fmt.Errorf("migration: failed to get all changes: %w", err)
		}
	} else {
		tm.dfs(ctx, heads, rootChange.Id)
	}
	newStorage, err := CreateStorage(ctx, rootChange, headStorage, store)
	if err != nil && !errors.Is(err, treestorage.ErrTreeExists) {
		return fmt.Errorf("migration: failed to create new storage: %w", err)
	}
	if errors.Is(err, treestorage.ErrTreeExists) {
		newStorage, err = NewStorage(ctx, rootChange.Id, headStorage, store)
		if err != nil {
			return fmt.Errorf("migration: failed to start old storage: %w", err)
		}
	}
	objTree, err := BuildMigratableObjectTree(newStorage, tm.aclList)
	if err != nil {
		return fmt.Errorf("migration: failed to build object tree: %w", err)
	}
	addPayload := RawChangesPayload{
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
		log.Errorf("migration: heads mismatch: %v, %v != %v", rootChange.Id, res.Heads, heads)
	}
	return nil
}

func (tm *TreeMigrator) dfs(ctx context.Context, heads []string, breakpoint string) (loadFailed bool) {
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
			loadFailed = true
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
	return
}

func (tm *TreeMigrator) loadChange(ctx context.Context, id string) (ch *Change, err error) {
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
