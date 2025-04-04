package keyvalue

import (
	"context"
	"errors"
	"io"

	"github.com/anyproto/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/keyvaluestorage/syncstorage"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/sync"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

var ErrUnexpectedMessageType = errors.New("unexpected message type")

const CName = "common.object.keyvalue"

type KeyValueService interface {
	app.ComponentRunnable
	DefaultStore() keyvaluestorage.Storage
	HandleMessage(ctx context.Context, msg objectmessages.HeadUpdate) (err error)
}

type keyValueService struct {
	storageId string
	spaceId   string
	ctx       context.Context
	cancel    context.CancelFunc

	spaceStorage  spacestorage.SpaceStorage
	defaultStore  keyvaluestorage.Storage
	clientFactory spacesyncproto.ClientFactory
	syncService   sync.SyncService
}

func (k *keyValueService) DefaultStore() keyvaluestorage.Storage {
	return k.defaultStore
}

func (k *keyValueService) SyncWithPeer(ctx context.Context, p peer.Peer) (err error) {
	if k.syncService == nil {
		return nil
	}
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
	for _, id := range append(newIds, changedIds...) {
		kv, err := innerStorage.GetKeyPeerId(ctx, id)
		if err != nil {
			return err
		}
		err = stream.Send(kv.Proto())
		if err != nil {
			return err
		}
	}
	for _, id := range append(theirChangedIds, removedIds...) {
		kv := &spacesyncproto.StoreKeyValue{
			KeyPeerId: id,
		}
		err := stream.Send(kv)
		if err != nil {
			return err
		}
	}
	var messages []*spacesyncproto.StoreKeyValue
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		messages = append(messages, msg)
	}
	return k.defaultStore.SetRaw(ctx, messages...)
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
	accountService := a.MustComponent(accountservice.CName).(accountservice.Service)
	aclList := a.MustComponent(syncacl.CName).(list.AclList)
	k.spaceStorage = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
	k.syncService = a.MustComponent(sync.CName).(sync.SyncService)
	k.storageId = storageIdFromSpace(k.spaceId)
	syncClient := syncstorage.New(spaceState.SpaceId, k.syncService)
	k.defaultStore, err = keyvaluestorage.New(
		k.ctx,
		k.storageId,
		k.spaceStorage.AnyStore(),
		k.spaceStorage.HeadStorage(),
		accountService.Account(),
		syncClient,
		aclList,
		keyvaluestorage.NoOpIndexer{})
	return
}

func (k *keyValueService) Name() (name string) {
	return CName
}

func (k *keyValueService) Run(ctx context.Context) (err error) {
	return nil
}

func (k *keyValueService) Close(ctx context.Context) (err error) {
	k.cancel()
	return nil
}

func storageIdFromSpace(spaceId string) (storageId string) {
	return spaceId + ".keyvalue"
}
