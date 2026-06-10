package keyvalue

import (
	"context"
	"errors"
	"sort"

	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/innerstorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/syncstorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/kvinterfaces"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/sync"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/util/cidutil"
)

var ErrUnexpectedMessageType = errors.New("unexpected message type")

// applyBatchSize bounds how many streamed values the client applies per SetRaw
// during a pull. Because the server streams values newest-first, applying in
// batches lets the most recent read-markers become queryable before the long
// tail finishes arriving.
const applyBatchSize = 100

var log = logger.NewNamed(kvinterfaces.CName)

type keyValueService struct {
	storageId string
	spaceId   string
	ctx       context.Context
	cancel    context.CancelFunc

	limiter       *concurrentLimiter
	defaultStore  keyvaluestorage.Storage
	clientFactory spacesyncproto.ClientFactory
}

func New() kvinterfaces.KeyValueService {
	return &keyValueService{}
}

func (k *keyValueService) DefaultStore() keyvaluestorage.Storage {
	return k.defaultStore
}

func (k *keyValueService) SyncWithPeer(p peer.Peer) (err error) {
	k.limiter.ScheduleRequest(k.ctx, p.Id(), func() {
		err = k.syncWithPeer(k.ctx, p)
		if err != nil {
			log.Error("failed to sync with peer", zap.String("peerId", p.Id()), zap.Error(err))
		}
	})
	return nil
}

func (k *keyValueService) syncWithPeer(ctx context.Context, p peer.Peer) (err error) {
	conn, err := p.AcquireDrpcConn(ctx)
	if err != nil {
		return
	}
	defer p.ReleaseDrpcConn(ctx, conn)
	var (
		client = k.clientFactory.Client(conn)
		rdiff  = NewRemoteDiff(k.spaceId, client)
		diff   = k.defaultStore.InnerStorage().Diff()
	)
	newIds, changedIds, theirChangedIds, removedIds, err := diff.CompareDiff(ctx, rdiff)
	err = rpcerr.Unwrap(err)
	if err != nil {
		return err
	}
	innerStorage := k.defaultStore.InnerStorage()
	stream, err := client.StoreElements(ctx)
	if err != nil {
		return err
	}
	defer stream.CloseSend()
	err = stream.Send(&spacesyncproto.StoreKeyValue{SpaceId: k.spaceId})
	if err != nil {
		return err
	}
	for _, id := range append(removedIds, changedIds...) {
		kv, err := innerStorage.GetKeyPeerId(ctx, id)
		if err != nil {
			return err
		}
		err = stream.Send(kv.Proto())
		if err != nil {
			return err
		}
	}
	for _, id := range append(theirChangedIds, newIds...) {
		kv := &spacesyncproto.StoreKeyValue{
			KeyPeerId: id,
		}
		err := stream.Send(kv)
		if err != nil {
			return err
		}
	}
	err = stream.Send(&spacesyncproto.StoreKeyValue{})
	if err != nil {
		return err
	}
	// Apply incrementally as values stream in (the server sends them newest-first)
	// so the most recent read-markers become queryable before the long tail
	// arrives. SetRaw is LWW-idempotent and each call is atomic, so batching is
	// safe and interrupt-safe: an aborted stream just leaves a consistent prefix
	// that the next sync resumes. Each Recv yields a fresh message and SetRaw does
	// not retain the batch, so reallocating it per flush is safe. (Trade-off: one
	// broadcast per batch instead of one for the whole pull — same value volume.)
	batch := make([]*spacesyncproto.StoreKeyValue, 0, applyBatchSize)
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		if msg.KeyPeerId == "" {
			break
		}
		batch = append(batch, msg)
		if len(batch) >= applyBatchSize {
			if err = k.defaultStore.SetRaw(ctx, batch...); err != nil {
				return err
			}
			batch = make([]*spacesyncproto.StoreKeyValue, 0, applyBatchSize)
		}
	}
	if len(batch) > 0 {
		return k.defaultStore.SetRaw(ctx, batch...)
	}
	return nil
}

func (k *keyValueService) HandleStoreDiffRequest(ctx context.Context, req *spacesyncproto.StoreDiffRequest) (resp *spacesyncproto.StoreDiffResponse, err error) {
	return HandleRangeRequest(ctx, k.defaultStore.InnerStorage().Diff(), req)
}

func (k *keyValueService) HandleStoreElementsRequest(ctx context.Context, stream spacesyncproto.DRPCSpaceSync_StoreElementsStream) (err error) {
	var (
		messagesToSave []*spacesyncproto.StoreKeyValue
		messagesToSend []string
	)
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		if msg.KeyPeerId == "" {
			break
		}
		if msg.Value != nil {
			messagesToSave = append(messagesToSave, msg)
		} else {
			messagesToSend = append(messagesToSend, msg.KeyPeerId)
		}
	}
	innerStorage := k.defaultStore.InnerStorage()
	// Fetch the requested values, then stream them newest-first (by "t" desc) so a
	// cold-syncing peer gets the most recent read-markers before the long tail.
	// SetRaw is LWW-idempotent, so order never changes the converged set — only
	// when each value lands. Each value's byte slices are copied because
	// GetKeyPeerId returns buffers that the next fetch reuses.
	kvs := make([]innerstorage.KeyValue, 0, len(messagesToSend))
	for _, id := range messagesToSend {
		kv, err := innerStorage.GetKeyPeerId(ctx, id)
		if err != nil {
			log.Warn("failed to get key value", zap.String("key", id), zap.Error(err))
			continue
		}
		kv.Value.Value = append([]byte(nil), kv.Value.Value...)
		kv.Value.PeerSignature = append([]byte(nil), kv.Value.PeerSignature...)
		kv.Value.IdentitySignature = append([]byte(nil), kv.Value.IdentitySignature...)
		kvs = append(kvs, kv)
	}
	// Newest-first. NB: despite its name, TimestampMilli holds the write time in
	// MICROSECONDS (set from StoreKeyInner.TimestampMicro), so this orders with
	// microsecond resolution.
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].TimestampMilli > kvs[j].TimestampMilli
	})
	isError := false
	for i := range kvs {
		if err = stream.Send(kvs[i].Proto()); err != nil {
			log.Warn("failed to send key value", zap.Error(err))
			isError = true
			break
		}
	}
	if !isError {
		if err = stream.Send(&spacesyncproto.StoreKeyValue{}); err != nil {
			return err
		}
	}
	// Persist the peer's pushed values regardless of any send error above: they
	// were already received and are independent of streaming our response.
	return k.defaultStore.SetRaw(ctx, messagesToSave...)
}

func (k *keyValueService) HandleMessage(ctx context.Context, headUpdate drpc.Message) (err error) {
	update, ok := headUpdate.(*objectmessages.HeadUpdate)
	if !ok {
		return ErrUnexpectedMessageType
	}
	keyValueMsg := &spacesyncproto.StoreKeyValues{}
	err = keyValueMsg.UnmarshalVT(update.Bytes)
	if err != nil {
		objectmessages.FreeHeadUpdate(update)
		return err
	}
	objectmessages.FreeHeadUpdate(update)
	return k.defaultStore.SetRaw(ctx, keyValueMsg.KeyValues...)
}

func (k *keyValueService) Init(a *app.App) (err error) {
	k.ctx, k.cancel = context.WithCancel(context.Background())
	spaceState := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	k.spaceId = spaceState.SpaceId
	k.clientFactory = spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceSyncClient)
	k.limiter = newConcurrentLimiter()
	accountService := a.MustComponent(accountservice.CName).(accountservice.Service)
	aclList := a.MustComponent(syncacl.CName).(list.AclList)
	spaceStorage := a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
	syncService := a.MustComponent(sync.CName).(sync.SyncService)
	k.storageId, err = storageIdFromSpace(k.spaceId)
	if err != nil {
		return err
	}
	indexer := a.Component(keyvaluestorage.IndexerCName).(keyvaluestorage.Indexer)
	if indexer == nil {
		indexer = keyvaluestorage.NoOpIndexer{}
	}
	syncClient := syncstorage.New(spaceState.SpaceId, syncService)
	k.defaultStore, err = keyvaluestorage.New(
		k.ctx,
		k.storageId,
		spaceStorage.AnyStore(),
		spaceStorage.HeadStorage(),
		accountService.Account(),
		syncClient,
		aclList,
		indexer)
	return
}

func (k *keyValueService) Name() (name string) {
	return kvinterfaces.CName
}

func (k *keyValueService) Run(ctx context.Context) (err error) {
	return k.defaultStore.Prepare()
}

func (k *keyValueService) Close(ctx context.Context) (err error) {
	k.cancel()
	k.limiter.Close()
	return nil
}

func storageIdFromSpace(spaceId string) (storageId string, err error) {
	header := &spacesyncproto.StorageHeader{
		SpaceId:     spaceId,
		StorageName: "default",
	}
	data, err := header.MarshalVT()
	if err != nil {
		return "", err
	}
	cid, err := cidutil.NewCidFromBytes(data)
	if err != nil {
		return "", err
	}
	return cid, nil
}
