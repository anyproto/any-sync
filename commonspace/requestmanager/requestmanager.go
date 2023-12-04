package requestmanager

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/debugstat"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/objectsync"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
)

const CName = "common.commonspace.requestmanager"

var log = logger.NewNamed(CName)

type RequestManager interface {
	app.ComponentRunnable
	SendRequest(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error)
	QueueRequest(peerId string, msg *spacesyncproto.ObjectSyncMessage) (err error)
}

func New() RequestManager {
	return &requestManager{
		workers:   10,
		queueSize: 300,
		pools:     map[string]*RequestPool{},
	}
}

type MessageHandler interface {
	HandleMessage(ctx context.Context, hm objectsync.HandleMessage) (err error)
}

type requestManager struct {
	sync.Mutex
	pools         map[string]*RequestPool
	peerPool      pool.Pool
	workers       int
	queueSize     int
	handler       MessageHandler
	ctx           context.Context
	cancel        context.CancelFunc
	clientFactory spacesyncproto.ClientFactory
	statService   debugstat.StatService
	reqStat       *requestStat
	spaceId       string
}

func (r *requestManager) AggregateStat(stats []debugstat.StatValue) any {
	return r.reqStat.Aggregate(stats)
}

func (r *requestManager) ProvideStat() any {
	return r.reqStat.QueueStat()
}

func (r *requestManager) StatId() string {
	return r.spaceId
}

func (r *requestManager) StatType() string {
	return CName
}

func (r *requestManager) Init(a *app.App) (err error) {
	r.ctx, r.cancel = context.WithCancel(context.Background())
	spaceState := a.MustComponent(spacestate.CName).(*spacestate.SpaceState)
	r.statService, _ = a.Component(debugstat.CName).(debugstat.StatService)
	if r.statService == nil {
		r.statService = debugstat.NewNoOp()
	}
	r.reqStat = newRequestStat(spaceState.SpaceId)
	r.spaceId = spaceState.SpaceId
	r.handler = a.MustComponent(objectsync.CName).(MessageHandler)
	r.peerPool = a.MustComponent(pool.CName).(pool.Pool)
	r.clientFactory = spacesyncproto.ClientFactoryFunc(spacesyncproto.NewDRPCSpaceSyncClient)
	return
}

func (r *requestManager) Name() (name string) {
	return CName
}

func (r *requestManager) Run(ctx context.Context) (err error) {
	r.statService.AddProvider(r)
	return nil
}

func (r *requestManager) Close(ctx context.Context) (err error) {
	r.Lock()
	defer r.Unlock()
	r.cancel()
	r.statService.RemoveProvider(r)
	for _, p := range r.pools {
		_ = p.Close()
	}
	return nil
}

func (r *requestManager) SendRequest(ctx context.Context, peerId string, req *spacesyncproto.ObjectSyncMessage) (reply *spacesyncproto.ObjectSyncMessage, err error) {
	// TODO: limit concurrent sends?
	r.reqStat.AddSyncRequest(peerId, req)
	defer func() {
		r.reqStat.RemoveSyncRequest(peerId, req)
	}()
	return r.doRequest(ctx, peerId, req)
}

func (r *requestManager) QueueRequest(peerId string, req *spacesyncproto.ObjectSyncMessage) (err error) {
	r.Lock()
	defer r.Unlock()
	pl, exists := r.pools[peerId]
	if !exists {
		pl = NewRequestPool(r.workers, r.queueSize)
		r.pools[peerId] = pl
		pl.Run()
	}
	r.reqStat.AddQueueRequest(peerId, req)
	// TODO: for later think when many clients are there,
	//  we need to close pools for inactive clients
	return pl.TryAdd(req.ObjectId, func() {
		doRequestAndHandle(r, peerId, req)
		r.reqStat.RemoveQueueRequest(peerId, req)
	})
}

var doRequestAndHandle = (*requestManager).requestAndHandle

func (r *requestManager) requestAndHandle(peerId string, req *spacesyncproto.ObjectSyncMessage) {
	ctx := r.ctx
	resp, err := r.doRequest(ctx, peerId, req)
	if err != nil {
		log.Warn("failed to send request", zap.Error(err))
		return
	}
	ctx = peer.CtxWithPeerId(ctx, peerId)
	_ = r.handler.HandleMessage(ctx, objectsync.HandleMessage{
		SenderId: peerId,
		Message:  resp,
		PeerCtx:  ctx,
	})
}

func (r *requestManager) doRequest(ctx context.Context, peerId string, msg *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error) {
	pr, err := r.peerPool.Get(ctx, peerId)
	if err != nil {
		return
	}
	err = pr.DoDrpc(ctx, func(conn drpc.Conn) error {
		cl := r.clientFactory.Client(conn)
		resp, err = cl.ObjectSync(ctx, msg)
		return err
	})
	err = rpcerr.Unwrap(err)
	return
}
