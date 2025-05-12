package migration

import (
	"context"
	"fmt"
	"slices"

	anystore "github.com/anyproto/any-store"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/commonspace/spacestorage/oldstorage"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

func migrateAclList(ctx context.Context, oldStorage oldstorage.ListStorage, headStorage headstorage.HeadStorage, store anystore.DB) (list.AclList, error) {
	rootChange, err := oldStorage.Root()
	if err != nil {
		return nil, fmt.Errorf("migration: failed to get acl root change: %w", err)
	}
	head, err := oldStorage.Head()
	if err != nil {
		return nil, fmt.Errorf("migration: failed to get acl head: %w", err)
	}
	aclStorage, err := list.NewStorage(ctx, rootChange.Id, headStorage, store)
	if err != nil {
		return nil, fmt.Errorf("migration: failed to create acl storage: %w", err)
	}
	keys, err := accountdata.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("migration: failed to generate keys: %w", err)
	}
	aclList, err := list.BuildAclListWithIdentity(keys, aclStorage, recordverifier.NewValidateFull())
	if err != nil {
		return nil, fmt.Errorf("migration: failed to build acl list: %w", err)
	}
	var (
		allRecords []*consensusproto.RawRecordWithId
		rec        *consensusproto.RawRecordWithId
		cur        = head
		builder    = aclList.RecordBuilder()
	)
	for rec == nil || rec.Id != rootChange.Id {
		rec, err = oldStorage.GetRawRecord(ctx, cur)
		if err != nil {
			return nil, fmt.Errorf("migration: failed to get acl record: %w", err)
		}
		allRecords = append(allRecords, rec)
		res, err := builder.UnmarshallWithId(rec)
		if err != nil {
			return nil, fmt.Errorf("migration: failed to unmarshall acl record: %w", err)
		}
		cur = res.PrevId
	}
	slices.Reverse(allRecords)
	err = aclList.AddRawRecords(allRecords)
	if err != nil {
		return nil, fmt.Errorf("migration: failed to add acl records: %w", err)
	}
	return aclList, nil
}
