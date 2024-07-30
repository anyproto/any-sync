package objectsync

import (
	"context"
	"errors"
	"fmt"

	"github.com/cheggaaa/mb/v3"
	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/util/multiqueue"
)

var ErrUnexpectedHeadUpdateType = errors.New("unexpected head update type")

type objectSync struct {
	spaceId string
	pool    pool.Service
	manager treemanager.TreeManager
	status  syncstatus.StatusUpdater
}

type peerIdSettable interface {
	SetPeerId(peerId string)
}

func New() syncdeps.SyncHandler {
	return &objectSync{}
}

func (o *objectSync) Init(a *app.App) (err error) {
	o.manager = a.MustComponent(treemanager.CName).(treemanager.TreeManager)
	o.pool = a.MustComponent(pool.CName).(pool.Service)
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
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	obj, err := o.manager.GetTree(context.Background(), update.Meta.SpaceId, update.Meta.ObjectId)
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
	obj, err := o.manager.GetTree(context.Background(), o.spaceId, rq.ObjectId())
	if err != nil {
		return synctree.NewRequest(rq.PeerId(), o.spaceId, rq.ObjectId(), nil, nil, nil), treechangeproto.ErrGetTree
	}
	objHandler, ok := obj.(syncdeps.ObjectSyncHandler)
	if !ok {
		return nil, fmt.Errorf("object %s does not support sync", obj.Id())
	}
	return objHandler.HandleStreamRequest(ctx, rq, updater, sendResponse)
}

func (o *objectSync) HandleDeprecatedObjectSync(ctx context.Context, req *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error) {
	obj, err := o.manager.GetTree(context.Background(), o.spaceId, req.ObjectId)
	if err != nil {
		unmarshalled := &treechangeproto.TreeSyncMessage{}
		err = proto.Unmarshal(req.Payload, unmarshalled)
		if err != nil {
			return nil, err
		}
		cnt := unmarshalled.GetContent().GetFullSyncRequest()
		// we also don't have the tree, so we can't handle this request
		if unmarshalled.RootChange == nil || cnt == nil {
			return nil, treechangeproto.ErrGetTree
		}
		// we don't have the tree, so we return empty response, so next time we will get the tree
		if cnt.Changes == nil {
			return &spacesyncproto.ObjectSyncMessage{
				SpaceId:  req.SpaceId,
				ObjectId: req.ObjectId,
			}, nil
		}
		// we don't have the tree, but this must be a request with full data
		payload := treestorage.TreeStorageCreatePayload{
			RootRawChange: unmarshalled.RootChange,
			Changes:       cnt.Changes,
			Heads:         cnt.Heads,
		}
		err := o.manager.ValidateAndPutTree(ctx, o.spaceId, payload)
		if err != nil {
			return nil, err
		}
		resp := &treechangeproto.TreeFullSyncResponse{
			Heads:        cnt.Heads,
			SnapshotPath: cnt.SnapshotPath,
		}
		syncMsg := treechangeproto.WrapFullResponse(resp, unmarshalled.RootChange)
		marshalled, err := proto.Marshal(syncMsg)
		if err != nil {
			return nil, err
		}
		return &spacesyncproto.ObjectSyncMessage{
			SpaceId:  req.SpaceId,
			ObjectId: req.ObjectId,
			Payload:  marshalled,
		}, nil
	}
	objHandler, ok := obj.(syncdeps.ObjectSyncHandler)
	if !ok {
		return nil, fmt.Errorf("object %s does not support sync", obj.Id())
	}
	return objHandler.HandleDeprecatedRequest(ctx, req)
}

func (o *objectSync) ApplyRequest(ctx context.Context, rq syncdeps.Request, requestSender syncdeps.RequestSender) error {
	ctx = peer.CtxWithPeerId(ctx, rq.PeerId())
	obj, err := o.manager.GetTree(context.Background(), o.spaceId, rq.ObjectId())
	// if tree exists locally
	if err == nil {
		objHandler, ok := obj.(syncdeps.ObjectSyncHandler)
		if !ok {
			return fmt.Errorf("object %s does not support sync", obj.Id())
		}
		collector := objHandler.ResponseCollector()
		return requestSender.SendRequest(ctx, rq, collector)
	}
	_, err = o.manager.GetTree(ctx, o.spaceId, rq.ObjectId())
	return err
}

func (o *objectSync) TryAddMessage(ctx context.Context, peerId string, msg multiqueue.Sizeable, q *mb.MB[multiqueue.Sizeable]) error {
	settable, ok := msg.(peerIdSettable)
	if ok {
		settable.SetPeerId(peerId)
	}
	return q.TryAdd(msg)
}

func (o *objectSync) SendStreamRequest(ctx context.Context, rq syncdeps.Request, receive func(stream drpc.Stream) error) (err error) {
	pr, err := o.pool.GetOneOf(ctx, []string{rq.PeerId()})
	if err != nil {
		return err
	}
	return pr.DoDrpc(ctx, func(conn drpc.Conn) error {
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
}

func (o *objectSync) NewMessage() drpc.Message {
	return &objectmessages.HeadUpdate{}
}
