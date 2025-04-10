package keyvalue

import (
	"context"
	"errors"

	"github.com/anyproto/protobuf/proto"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
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
	defer p.ReleaseDrpcConn(conn)
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
	var messages []*spacesyncproto.StoreKeyValue
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		if msg.KeyPeerId == "" {
			break
		}
		messages = append(messages, msg)
	}
	return k.defaultStore.SetRaw(ctx, messages...)
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
	isError := false
	for _, msg := range messagesToSend {
		kv, err := innerStorage.GetKeyPeerId(ctx, msg)
		if err != nil {
			log.Warn("failed to get key value", zap.String("key", msg), zap.Error(err))
			continue
		}
		err = stream.Send(kv.Proto())
		if err != nil {
			log.Warn("failed to send key value", zap.String("key", msg), zap.Error(err))
			isError = true
			break
		}
	}
	if !isError {
		err = stream.Send(&spacesyncproto.StoreKeyValue{})
		if err != nil {
			return err
		}
	}
	return k.defaultStore.SetRaw(ctx, messagesToSave...)
}

func (k *keyValueService) HandleMessage(ctx context.Context, headUpdate drpc.Message) (err error) {
	update, ok := headUpdate.(*objectmessages.HeadUpdate)
	if !ok {
		return ErrUnexpectedMessageType
	}
	keyValueMsg := &spacesyncproto.StoreKeyValues{}
	err = proto.Unmarshal(update.Bytes, keyValueMsg)
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
	data, err := proto.Marshal(header)
	if err != nil {
		return "", err
	}
	cid, err := cidutil.NewCidFromBytes(data)
	if err != nil {
		return "", err
	}
	return cid, nil
}
