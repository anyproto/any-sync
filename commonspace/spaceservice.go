package commonspace

import (
	"context"
	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/config"
	"github.com/anyproto/any-sync/commonspace/credentialprovider"
	"github.com/anyproto/any-sync/commonspace/deletionstate"
	"github.com/anyproto/any-sync/commonspace/headsync"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
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
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc/rpcerr"
	"github.com/anyproto/any-sync/nodeconf"
	"storj.io/drpc"
	"sync/atomic"
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
	NewSpace(ctx context.Context, id string) (sp Space, err error)
	app.Component
}

type spaceService struct {
	config               config.Config
	account              accountservice.Service
	configurationService nodeconf.Service
	storageProvider      spacestorage.SpaceStorageProvider
	peermanagerProvider  peermanager.PeerManagerProvider
	credentialProvider   credentialprovider.CredentialProvider
	treeManager          treemanager.TreeManager
	pool                 pool.Pool
	metric               metric.Metric
	app                  *app.App
}

func (s *spaceService) Init(a *app.App) (err error) {
	s.config = a.MustComponent("config").(config.ConfigGetter).GetSpace()
	s.account = a.MustComponent(accountservice.CName).(accountservice.Service)
	s.storageProvider = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorageProvider)
	s.configurationService = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.treeManager = a.MustComponent(treemanager.CName).(treemanager.TreeManager)
	s.peermanagerProvider = a.MustComponent(peermanager.CName).(peermanager.PeerManagerProvider)
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

func (s *spaceService) NewSpace(ctx context.Context, id string) (Space, error) {
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
	var (
		spaceIsClosed  = &atomic.Bool{}
		spaceIsDeleted = &atomic.Bool{}
	)
	isDeleted, err := st.IsSpaceDeleted()
	if err != nil {
		return nil, err
	}
	spaceIsDeleted.Swap(isDeleted)
	state := &spacestate.SpaceState{
		SpaceId:        st.Id(),
		SpaceIsDeleted: spaceIsDeleted,
		SpaceIsClosed:  spaceIsClosed,
		TreesUsed:      &atomic.Int32{},
	}
	if s.config.KeepTreeDataInMemory {
		state.TreeBuilderFunc = objecttree.BuildObjectTree
	} else {
		state.TreeBuilderFunc = objecttree.BuildEmptyDataObjectTree
	}
	peerManager, err := s.peermanagerProvider.NewPeerManager(ctx, id)
	if err != nil {
		return nil, err
	}
	spaceApp := s.app.ChildApp()
	spaceApp.Register(state).
		Register(peerManager).
		Register(newCommonStorage(st)).
		Register(syncacl.New()).
		Register(requestmanager.New()).
		Register(deletionstate.New()).
		Register(settings.New()).
		Register(objectmanager.New(s.treeManager)).
		Register(objecttreebuilder.New()).
		Register(objectsync.New()).
		Register(headsync.New())

	sp := &space{
		state: state,
		app:   spaceApp,
	}
	return sp, nil
}

func (s *spaceService) addSpaceStorage(ctx context.Context, spaceDescription SpaceDescription) (st spacestorage.SpaceStorage, err error) {
	payload := spacestorage.SpaceStorageCreatePayload{
		AclWithId: &aclrecordproto.RawAclRecordWithId{
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
	var p peer.Peer
	lastConfiguration := s.configurationService
	// we can't connect to client if it is a node
	if lastConfiguration.IsResponsible(id) {
		err = spacesyncproto.ErrSpaceMissing
		return
	}

	p, err = s.pool.GetOneOf(ctx, lastConfiguration.NodeIds(id))
	if err != nil {
		return
	}

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

	st, err = s.createSpaceStorage(spacestorage.SpaceStorageCreatePayload{
		AclWithId: &aclrecordproto.RawAclRecordWithId{
			Payload: res.Payload.AclPayload,
			Id:      res.Payload.AclPayloadId,
		},
		SpaceSettingsWithId: &treechangeproto.RawTreeChangeWithId{
			RawChange: res.Payload.SpaceSettingsPayload,
			Id:        res.Payload.SpaceSettingsPayloadId,
		},
		SpaceHeaderWithId: res.Payload.SpaceHeader,
	})
	return
}

func (s *spaceService) createSpaceStorage(payload spacestorage.SpaceStorageCreatePayload) (spacestorage.SpaceStorage, error) {
	err := validateSpaceStorageCreatePayload(payload)
	if err != nil {
		return nil, err
	}
	return s.storageProvider.CreateSpaceStorage(payload)
}
