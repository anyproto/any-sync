package commonspace

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/object/treesyncer"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/util/periodicsync"
)

func NewTreeSyncer(spaceId string) treesyncer.TreeSyncer {
	return &treeSyncer{spaceId: spaceId}
}

type treeSyncer struct {
	spaceId     string
	treeManager treemanager.TreeManager
}

func (t *treeSyncer) Init(a *app.App) (err error) {
	t.treeManager = a.MustComponent(treemanager.CName).(treemanager.TreeManager)
	return
}

func (t *treeSyncer) Name() (name string) {
	return treesyncer.CName
}

func (t *treeSyncer) Run(ctx context.Context) (err error) {
	return nil
}

func (t *treeSyncer) Close(ctx context.Context) (err error) {
	return nil
}

func (t *treeSyncer) StartSync() {
}

func (t *treeSyncer) StopSync() {
}

func (t *treeSyncer) ShouldSync(peerId string) bool {
	return true
}

func (t *treeSyncer) SyncAll(ctx context.Context, p peer.Peer, existing, missing []string) (err error) {
	syncTrees := func(ids []string) {
		for _, id := range ids {
			log := log.With(zap.String("treeId", id))
			tr, err := t.treeManager.GetTree(ctx, t.spaceId, id)
			if err != nil {
				log.WarnCtx(ctx, "can't load existing tree", zap.Error(err))
				return
			}
			syncTree, ok := tr.(synctree.SyncTree)
			if !ok {
				log.WarnCtx(ctx, "not a sync tree")
			}
			if err = syncTree.SyncWithPeer(ctx, p); err != nil {
				log.WarnCtx(ctx, "synctree.SyncWithPeer error", zap.Error(err))
			} else {
				log.DebugCtx(ctx, "success *synctree.SyncWithPeer")
			}
		}
	}
	syncTrees(missing)
	syncTrees(existing)
	return
}

const RpcName = "rpcserver"

type RpcServer struct {
	spaceService SpaceService
	spaces       map[string]Space
	streamPool   streampool.StreamPool
	sync.Mutex
}

func NewRpcServer() *RpcServer {
	return &RpcServer{
		spaces: make(map[string]Space),
	}
}

type failingStream struct {
	spacesyncproto.DRPCSpaceSync_ObjectSyncStreamStream
	tryFail bool
}

func (f *failingStream) Send(msg *spacesyncproto.ObjectSyncMessage) error {
	if f.tryFail && rand.Uint32()%2 == 0 {
		return nil
	}
	return f.DRPCSpaceSync_ObjectSyncStreamStream.Send(msg)
}

func (r *RpcServer) getSpace(ctx context.Context, spaceId string) (sp Space, err error) {
	r.Lock()
	defer r.Unlock()
	sp, ok := r.spaces[spaceId]
	if !ok {
		sp, err = r.spaceService.NewSpace(ctx, spaceId, Deps{
			TreeSyncer: NewTreeSyncer(spaceId),
			SyncStatus: syncstatus.NewNoOpSyncStatus(),
		})
		if err != nil {
			return nil, err
		}
		err = sp.Init(ctx)
		if err != nil {
			return nil, err
		}
		r.spaces[spaceId] = sp
	}
	return sp, nil
}

func (r *RpcServer) GetSpace(ctx context.Context, spaceId string) (Space, error) {
	return r.getSpace(ctx, spaceId)
}

func (r *RpcServer) HeadSync(ctx context.Context, request *spacesyncproto.HeadSyncRequest) (*spacesyncproto.HeadSyncResponse, error) {
	sp, err := r.getSpace(ctx, request.SpaceId)
	if err != nil {
		return nil, err
	}
	resp, err := sp.HandleRangeRequest(ctx, request)
	return resp, err
}

func (r *RpcServer) SpacePush(ctx context.Context, request *spacesyncproto.SpacePushRequest) (*spacesyncproto.SpacePushResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RpcServer) SpacePull(ctx context.Context, request *spacesyncproto.SpacePullRequest) (*spacesyncproto.SpacePullResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RpcServer) ObjectSyncStream(stream spacesyncproto.DRPCSpaceSync_ObjectSyncStreamStream) error {
	return r.streamPool.ReadStream(&failingStream{stream, false}, 100)
}

func (r *RpcServer) ObjectSync(ctx context.Context, message *spacesyncproto.ObjectSyncMessage) (*spacesyncproto.ObjectSyncMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RpcServer) ObjectSyncRequestStream(message *spacesyncproto.ObjectSyncMessage, stream spacesyncproto.DRPCSpaceSync_ObjectSyncRequestStreamStream) error {
	sp, err := r.getSpace(stream.Context(), message.SpaceId)
	if err != nil {
		return err
	}
	return sp.HandleStreamSyncRequest(stream.Context(), message, stream)
}

func (r *RpcServer) AclAddRecord(ctx context.Context, request *spacesyncproto.AclAddRecordRequest) (*spacesyncproto.AclAddRecordResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RpcServer) AclGetRecords(ctx context.Context, request *spacesyncproto.AclGetRecordsRequest) (*spacesyncproto.AclGetRecordsResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RpcServer) Init(a *app.App) (err error) {
	serv := a.MustComponent(server.CName).(*rpctest.TestServer)
	r.spaceService = a.MustComponent(CName).(SpaceService)
	r.streamPool = a.MustComponent(streampool.CName).(streampool.StreamPool)
	return spacesyncproto.DRPCRegisterSpaceSync(serv, r)
}

func (r *RpcServer) Name() (name string) {
	return RpcName
}

const SpaceProcessName = "spaceprocess"

type spaceProcess struct {
	spaceId        string
	spaceServer    *RpcServer
	accountService accountservice.Service
	manager        *testTreeManager
	periodicCall   periodicsync.PeriodicSync
}

func newSpaceProcess(spaceId string) *spaceProcess {
	return &spaceProcess{spaceId: spaceId}
}

func (s *spaceProcess) Init(a *app.App) (err error) {
	s.manager = a.MustComponent(treemanager.CName).(*testTreeManager)
	s.spaceServer = a.MustComponent(RpcName).(*RpcServer)
	s.accountService = a.MustComponent(accountservice.CName).(accountservice.Service)
	s.periodicCall = periodicsync.NewPeriodicSyncDuration(50*time.Millisecond, 0, s.update, log)
	return
}

func (s *spaceProcess) Name() (name string) {
	return SpaceProcessName
}

func (s *spaceProcess) Run(ctx context.Context) (err error) {
	s.periodicCall.Run()
	return nil
}

func (s *spaceProcess) Close(ctx context.Context) (err error) {
	s.periodicCall.Close()
	return nil
}

func (s *spaceProcess) update(ctx context.Context) error {
	sp, err := s.spaceServer.GetSpace(ctx, s.spaceId)
	if err != nil {
		return err
	}
	var tr objecttree.ObjectTree
	newDoc := rand.Int()%20 == 0
	snapshot := rand.Int()%10 == 0
	allTrees := sp.StoredIds()
	if newDoc || len(allTrees) == 0 {
		tr, err = s.manager.CreateTree(ctx, s.spaceId)
		if err != nil {
			return err
		}
	} else {
		rnd := rand.Int() % len(allTrees)
		tr, err = s.manager.GetTree(ctx, s.spaceId, allTrees[rnd])
		if err != nil {
			return err
		}
	}
	tr.Lock()
	defer tr.Unlock()
	bytes := make([]byte, 1024)
	_, _ = rand.Read(bytes)
	_, err = tr.AddContent(ctx, objecttree.SignableChangeContent{
		Data:        bytes,
		Key:         s.accountService.Account().SignKey,
		IsSnapshot:  snapshot,
		IsEncrypted: true,
		Timestamp:   0,
		DataType:    "",
	})
	return err
}
