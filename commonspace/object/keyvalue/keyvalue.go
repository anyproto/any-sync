package keyvalue

import (
	"context"
	"errors"
	"slices"
	"strings"

	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/ldiff"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage"
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

// SyncWithPeer only schedules the sync: it always returns nil, and any sync
// error is reported by the scheduled goroutine via the log. The goroutine must
// not write the named return — that would race with this function returning.
func (k *keyValueService) SyncWithPeer(p peer.Peer) (err error) {
	k.limiter.ScheduleRequest(k.ctx, p.Id(), func() {
		if syncErr := k.syncWithPeer(k.ctx, p); syncErr != nil {
			log.Error("failed to sync with peer", zap.String("peerId", p.Id()), zap.Error(syncErr))
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
	// so the most recent values become queryable before the long tail arrives.
	// SetRaw is LWW-idempotent and atomic per call, so an aborted pull just
	// leaves a consistent prefix that the next sync resumes. SetRaw does not
	// retain the batch slice, so it is reused between flushes.
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
			batch = batch[:0]
		}
	}
	return k.defaultStore.SetRaw(ctx, batch...)
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
	// Stream the requested values newest-first so a cold-syncing peer gets the
	// most recent values (e.g. read-markers) before the long tail. SetRaw is
	// LWW-idempotent, so order never changes the converged set — only when each
	// value lands. Sorting needs only the ids (the diff stores each element's
	// write timestamp in its Head), so values are fetched and sent one at a
	// time, keeping server memory flat regardless of how much is requested.
	sortIdsNewestFirst(innerStorage.Diff(), messagesToSend)
	var sendErr error
	for _, id := range messagesToSend {
		kv, err := innerStorage.GetKeyPeerId(ctx, id)
		if err != nil {
			log.Warn("failed to get key value", zap.String("key", id), zap.Error(err))
			continue
		}
		if sendErr = stream.Send(kv.Proto()); sendErr != nil {
			log.Warn("failed to send key value", zap.String("key", id), zap.Error(sendErr))
			break
		}
	}
	if sendErr == nil {
		sendErr = stream.Send(&spacesyncproto.StoreKeyValue{})
	}
	// Persist the peer's pushed values regardless of any send error above: they
	// were already received and are independent of streaming our response.
	if err = k.defaultStore.SetRaw(ctx, messagesToSave...); err != nil {
		return err
	}
	return sendErr
}

// sortIdsNewestFirst orders ids by write timestamp descending without loading
// any values: the diff stores each element's timestamp big-endian in its Head,
// so comparing Heads compares timestamps. Equal timestamps tie-break by id and
// ids missing from the diff sort last, making the order deterministic. The
// timestamps are writer-supplied — the same trust LWW conflict resolution
// already places in them — so newest-first is best-effort, not authoritative.
func sortIdsNewestFirst(diff ldiff.Diff, ids []string) {
	heads := make(map[string]string, len(ids))
	for _, id := range ids {
		if el, err := diff.Element(id); err == nil {
			heads[id] = el.Head
		}
	}
	slices.SortFunc(ids, func(a, b string) int {
		if c := strings.Compare(heads[b], heads[a]); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
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
