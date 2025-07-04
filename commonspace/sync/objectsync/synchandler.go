package objectsync

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"

	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/kvinterfaces"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/objectmanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

var ErrUnexpectedHeadUpdateType = errors.New("unexpected head update type")

var log = logger.NewNamed(syncdeps.CName)

type objectSync struct {
	spaceId  string
	pool     pool.Service
	manager  objectmanager.ObjectManager
	status   syncstatus.StatusUpdater
	keyValue kvinterfaces.KeyValueService
}

func New() syncdeps.SyncHandler {
	return &objectSync{}
}

func (o *objectSync) Init(a *app.App) (err error) {
	o.manager = a.MustComponent(treemanager.CName).(objectmanager.ObjectManager)
	o.pool = a.MustComponent(pool.CName).(pool.Service)
	o.keyValue = a.MustComponent(kvinterfaces.CName).(kvinterfaces.KeyValueService)
	o.status = a.MustComponent(syncstatus.CName).(syncstatus.StatusUpdater)
	o.spaceId = a.MustComponent(spacestate.CName).(*spacestate.SpaceState).SpaceId
	return
}

func (o *objectSync) Name() (name string) {
	return syncdeps.CName
}

func (o *objectSync) HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (syncdeps.Request, error) {
	update, ok := headUpdate.(*objectmessages.HeadUpdate)
	if !ok {
		return nil, ErrUnexpectedHeadUpdateType
	}
	if update.ObjectType() == spacesyncproto.ObjectType_KeyValue {
		return nil, o.keyValue.HandleMessage(ctx, update)
	}
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	obj, err := o.manager.GetObject(context.Background(), update.Meta.ObjectId)
	if err != nil {
		return synctree.NewRequest(peerId, update.Meta.SpaceId, update.Meta.ObjectId, nil, nil, nil), nil
	}
	objHandler, ok := obj.(syncdeps.ObjectSyncHandler)
	if !ok {
		return nil, fmt.Errorf("object %s does not support sync", obj.Id())
	}
	return objHandler.HandleHeadUpdate(ctx, o.status, update)
}

func (o *objectSync) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, updater syncdeps.QueueSizeUpdater, sendResponse func(resp proto.Message) error) (syncdeps.Request, error) {
	obj, err := o.manager.GetObject(context.Background(), rq.ObjectId())
	if err != nil {
		log.Debug("handle stream request no object", zap.String("spaceId", o.spaceId), zap.String("peerId", rq.PeerId()), zap.String("objectId", rq.ObjectId()))
		req, ok := rq.(*objectmessages.Request)
		if !ok {
			return nil, treechangeproto.ErrGetTree
		}
		treeSyncMsg := &treechangeproto.TreeSyncMessage{}
		err := treeSyncMsg.UnmarshalVT(req.Bytes)
		if err != nil {
			return nil, treechangeproto.ErrGetTree
		}
		request := treeSyncMsg.GetContent().GetFullSyncRequest()
		if request == nil || len(request.Heads) == 0 {
			return nil, treechangeproto.ErrGetTree
		}
		return synctree.NewRequest(rq.PeerId(), o.spaceId, rq.ObjectId(), nil, nil, nil), treechangeproto.ErrGetTree
	}
	objHandler, ok := obj.(syncdeps.ObjectSyncHandler)
	if !ok {
		return nil, fmt.Errorf("object %s does not support sync", obj.Id())
	}
	return objHandler.HandleStreamRequest(ctx, rq, updater, sendResponse)
}

func (o *objectSync) ApplyRequest(ctx context.Context, rq syncdeps.Request, requestSender syncdeps.RequestSender) error {
	ctx = peer.CtxWithPeerId(ctx, rq.PeerId())
	obj, err := o.manager.GetObject(context.Background(), rq.ObjectId())
	// if tree or acl exists locally
	if err == nil {
		objHandler, ok := obj.(syncdeps.ObjectSyncHandler)
		if !ok {
			return fmt.Errorf("object %s does not support sync", obj.Id())
		}
		collector := objHandler.ResponseCollector()
		return requestSender.SendRequest(ctx, rq, collector)
	}
	_, err = o.manager.GetObject(ctx, rq.ObjectId())
	return err
}

func (o *objectSync) SendStreamRequest(ctx context.Context, rq syncdeps.Request, receive func(stream drpc.Stream) error) (err error) {
	pr, err := o.pool.GetOneOf(ctx, []string{rq.PeerId()})
	if err != nil {
		return err
	}
	err = pr.DoDrpc(ctx, func(conn drpc.Conn) error {
		cl := spacesyncproto.NewDRPCSpaceSyncClient(conn)
		res, err := rq.Proto()
		if err != nil {
			return err
		}
		msg, ok := res.(*spacesyncproto.ObjectSyncMessage)
		if !ok {
			return fmt.Errorf("unexpected message type %T", res)
		}
		stream, err := cl.ObjectSyncRequestStream(ctx, msg)
		if err != nil {
			return err
		}
		return receive(stream)
	})
	if err != nil {
		err = rpcerr.Unwrap(err)
		return
	}
	return
}
