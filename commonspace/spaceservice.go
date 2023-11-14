//go:generate mockgen -destination mock_commonspace/mock_commonspace.go github.com/anyproto/any-sync/commonspace Space
package commonspace

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/commonspace/deletionmanager"
	"github.com/anyproto/any-sync/commonspace/object/treesyncer"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/net/peer"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/config"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/headsync"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/object/treemanager"
	"github.com/anyproto/any-sync/commonspace/objectmanager"
	"github.com/anyproto/any-sync/commonspace/objectsync"
	"github.com/anyproto/any-sync/commonspace/objecttreebuilder"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/requestmanager"
	"github.com/anyproto/any-sync/commonspace/settings"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"
)

const CName = "common.commonspace"

var log = logger.NewNamed(CName)

func New() SpaceService {
	return &spaceService{}
}

type ctxKey int

const AddSpaceCtxKey ctxKey = 0

type SpaceService interface {
	DeriveSpace(ctx context.Context, payload SpaceDerivePayload) (string, error)
	DeriveId(ctx context.Context, payload SpaceDerivePayload) (string, error)
	CreateSpace(ctx context.Context, payload SpaceCreatePayload) (string, error)
	NewSpace(ctx context.Context, id string, deps Deps) (sp Space, err error)
	app.Component
}

type Deps struct {
	TreeSyncer treesyncer.TreeSyncer
}

type spaceService struct {
	config                config.Config
	account               accountservice.Service
	configurationService  nodeconf.Service
	storageProvider       spacestorage.SpaceStorageProvider
	peerManagerProvider   peermanager.PeerManagerProvider
	credentialProvider    credentialprovider.CredentialProvider
	statusServiceProvider syncstatus.StatusServiceProvider
	treeManager           treemanager.TreeManager
	pool                  pool.Pool
	metric                metric.Metric
	app                   *app.App
}

func (s *spaceService) Init(a *app.App) (err error) {
	s.config = a.MustComponent("config").(config.ConfigGetter).GetSpace()
	s.account = a.MustComponent(accountservice.CName).(accountservice.Service)
	s.storageProvider = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorageProvider)
	s.configurationService = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.treeManager = a.MustComponent(treemanager.CName).(treemanager.TreeManager)
	s.peerManagerProvider = a.MustComponent(peermanager.CName).(peermanager.PeerManagerProvider)
	s.statusServiceProvider = a.MustComponent(syncstatus.CName).(syncstatus.StatusServiceProvider)
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.metric, _ = a.Component(metric.CName).(metric.Metric)
	s.app = a
	return nil
}

func (s *spaceService) Name() (name string) {
	return CName
}

func (s *spaceService) CreateSpace(ctx context.Context, payload SpaceCreatePayload) (id string, err error) {
	storageCreate, err := storagePayloadForSpaceCreate(payload)
	if err != nil {
		return
	}
	store, err := s.createSpaceStorage(storageCreate)
	if err != nil {
		if err == spacestorage.ErrSpaceStorageExists {
			return storageCreate.SpaceHeaderWithId.Id, nil
		}
		return
	}

	return store.Id(), nil
}

func (s *spaceService) DeriveId(ctx context.Context, payload SpaceDerivePayload) (id string, err error) {
	storageCreate, err := storagePayloadForSpaceDerive(payload)
	if err != nil {
		return
	}
	id = storageCreate.SpaceHeaderWithId.Id
	return
}

func (s *spaceService) DeriveSpace(ctx context.Context, payload SpaceDerivePayload) (id string, err error) {
	storageCreate, err := storagePayloadForSpaceDerive(payload)
	if err != nil {
		return
	}
	store, err := s.createSpaceStorage(storageCreate)
	if err != nil {
		if err == spacestorage.ErrSpaceStorageExists {
			return storageCreate.SpaceHeaderWithId.Id, nil
		}
		return
	}

	return store.Id(), nil
}

func (s *spaceService) NewSpace(ctx context.Context, id string, deps Deps) (Space, error) {
	st, err := s.storageProvider.WaitSpaceStorage(ctx, id)
	if err != nil {
		if err != spacestorage.ErrSpaceStorageMissing {
			return nil, err
		}

		if description, ok := ctx.Value(AddSpaceCtxKey).(SpaceDescription); ok {
			st, err = s.addSpaceStorage(ctx, description)
			if err != nil {
				return nil, err
			}
		} else {
			st, err = s.getSpaceStorageFromRemote(ctx, id)
			if err != nil {
				return nil, err
			}
		}
	}
	spaceIsClosed := &atomic.Bool{}
	state := &spacestate.SpaceState{
		SpaceId:       st.Id(),
		SpaceIsClosed: spaceIsClosed,
		TreesUsed:     &atomic.Int32{},
	}
	if s.config.KeepTreeDataInMemory {
		state.TreeBuilderFunc = objecttree.BuildObjectTree
	} else {
		state.TreeBuilderFunc = objecttree.BuildEmptyDataObjectTree
	}
	peerManager, err := s.peerManagerProvider.NewPeerManager(ctx, id)
	if err != nil {
		return nil, err
	}
	statusService := s.statusServiceProvider.NewStatusService()
	spaceApp := s.app.ChildApp()
	spaceApp.Register(state).
		Register(peerManager).
		Register(newCommonStorage(st)).
		Register(statusService).
		Register(syncacl.New()).
		Register(requestmanager.New()).
		Register(deletionstate.New()).
		Register(deletionmanager.New()).
		Register(settings.New()).
		Register(objectmanager.New(s.treeManager)).
		Register(deps.TreeSyncer).
		Register(objecttreebuilder.New()).
		Register(objectsync.New()).
		Register(headsync.New())

	sp := &space{
		state:   state,
		app:     spaceApp,
		storage: st,
	}
	return sp, nil
}

func (s *spaceService) addSpaceStorage(ctx context.Context, spaceDescription SpaceDescription) (st spacestorage.SpaceStorage, err error) {
	payload := spacestorage.SpaceStorageCreatePayload{
		AclWithId: &consensusproto.RawRecordWithId{
			Payload: spaceDescription.AclPayload,
			Id:      spaceDescription.AclId,
		},
		SpaceHeaderWithId: spaceDescription.SpaceHeader,
		SpaceSettingsWithId: &treechangeproto.RawTreeChangeWithId{
			RawChange: spaceDescription.SpaceSettingsPayload,
			Id:        spaceDescription.SpaceSettingsId,
		},
	}
	st, err = s.createSpaceStorage(payload)
	if err != nil {
		err = spacesyncproto.ErrUnexpected
		if err == spacestorage.ErrSpaceStorageExists {
			err = spacesyncproto.ErrSpaceExists
		}
		return
	}
	return
}

func (s *spaceService) getSpaceStorageFromRemote(ctx context.Context, id string) (st spacestorage.SpaceStorage, err error) {
	// we can't connect to client if it is a node
	if s.configurationService.IsResponsible(id) {
		err = spacesyncproto.ErrSpaceMissing
		return
	}

	pm, err := s.peerManagerProvider.NewPeerManager(ctx, id)
	if err != nil {
		return nil, err
	}
	var peers []peer.Peer
	for {
		peers, err = pm.GetResponsiblePeers(ctx)
		if err != nil && !errors.Is(err, net.ErrUnableToConnect) {
			return nil, err
		}
		if len(peers) == 0 {
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		} else {
			break
		}
	}
	for i, p := range peers {
		if st, err = s.spacePullWithPeer(ctx, p, id); err != nil {
			if i+1 == len(peers) {
				return
			} else {
				log.InfoCtx(ctx, "unable to pull space", zap.String("spaceId", id), zap.String("peerId", p.Id()))
			}
		} else {
			return
		}
	}
	return nil, net.ErrUnableToConnect
}

func (s *spaceService) spacePullWithPeer(ctx context.Context, p peer.Peer, id string) (st spacestorage.SpaceStorage, err error) {
	var res *spacesyncproto.SpacePullResponse
	err = p.DoDrpc(ctx, func(conn drpc.Conn) error {
		cl := spacesyncproto.NewDRPCSpaceSyncClient(conn)
		res, err = cl.SpacePull(ctx, &spacesyncproto.SpacePullRequest{Id: id})
		return err
	})
	if err != nil {
		err = rpcerr.Unwrap(err)
		return
	}

	return s.createSpaceStorage(spacestorage.SpaceStorageCreatePayload{
		AclWithId: &consensusproto.RawRecordWithId{
			Payload: res.Payload.AclPayload,
			Id:      res.Payload.AclPayloadId,
		},
		SpaceSettingsWithId: &treechangeproto.RawTreeChangeWithId{
			RawChange: res.Payload.SpaceSettingsPayload,
			Id:        res.Payload.SpaceSettingsPayloadId,
		},
		SpaceHeaderWithId: res.Payload.SpaceHeader,
	})
}

func (s *spaceService) createSpaceStorage(payload spacestorage.SpaceStorageCreatePayload) (spacestorage.SpaceStorage, error) {
	err := validateSpaceStorageCreatePayload(payload)
	if err != nil {
		return nil, err
	}
	return s.storageProvider.CreateSpaceStorage(payload)
}
