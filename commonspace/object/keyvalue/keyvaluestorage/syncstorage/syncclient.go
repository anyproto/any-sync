package syncstorage

import (
	"context"
	"fmt"

	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/sync"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
)

type innerUpdate struct {
	prepared  []byte
	keyValues []innerstorage.KeyValue
}

func (i *innerUpdate) Marshall(data objectmessages.ObjectMeta) ([]byte, error) {
	if i.prepared != nil {
		return i.prepared, nil
	}
	return nil, fmt.Errorf("no prepared data")
}

func (i *innerUpdate) Prepare() error {
	// TODO: Add peer to ignored peers list
	var (
		protoKeyValues []*spacesyncproto.StoreKeyValue
		err            error
	)
	for _, kv := range i.keyValues {
		protoKeyValues = append(protoKeyValues, kv.Proto())
	}
	keyValues := &spacesyncproto.StoreKeyValues{KeyValues: protoKeyValues}
	i.prepared, err = keyValues.Marshal()
	return err
}

func (i *innerUpdate) Heads() []string {
	return nil
}

func (i *innerUpdate) MsgSize() uint64 {
	return uint64(len(i.prepared))
}

func (i *innerUpdate) ObjectType() spacesyncproto.ObjectType {
	return spacesyncproto.ObjectType_KeyValue
}

type SyncClient interface {
	Broadcast(ctx context.Context, objectId string, keyValues ...innerstorage.KeyValue) error
}

type syncClient struct {
	spaceId     string
	syncService sync.SyncService
}

func New(spaceId string, syncService sync.SyncService) SyncClient {
	return &syncClient{
		spaceId:     spaceId,
		syncService: syncService,
	}
}

func (s *syncClient) Broadcast(ctx context.Context, objectId string, keyValue ...innerstorage.KeyValue) error {
	inner := &innerUpdate{
		keyValues: keyValue,
	}
	headUpdate := &objectmessages.HeadUpdate{
		Meta: objectmessages.ObjectMeta{
			ObjectId: objectId,
			SpaceId:  s.spaceId,
		},
		Update: inner,
	}
	return s.syncService.BroadcastMessage(ctx, headUpdate)
}
