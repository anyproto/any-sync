package commonspace

import (
	"context"
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/credentialprovider"
	"github.com/anytypeio/any-sync/commonspace/headsync"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/treemanager"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
	"github.com/anytypeio/any-sync/commonspace/peermanager"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/metric"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/net/pool"
	"github.com/anytypeio/any-sync/net/rpc/rpcerr"
	"github.com/anytypeio/any-sync/nodeconf"
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
	config               Config
	account              accountservice.Service
	configurationService nodeconf.Service
	storageProvider      spacestorage.SpaceStorageProvider
	peermanagerProvider  peermanager.PeerManagerProvider
	credentialProvider   credentialprovider.CredentialProvider
	treeManager          treemanager.TreeManager
	pool                 pool.Pool
	metric               metric.Metric
}

func (s *spaceService) Init(a *app.App) (err error) {
	s.config = a.MustComponent("config").(ConfigGetter).GetSpace()
	s.account = a.MustComponent(accountservice.CName).(accountservice.Service)
	s.storageProvider = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorageProvider)
	s.configurationService = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.treeManager = a.MustComponent(treemanager.CName).(treemanager.TreeManager)
	s.peermanagerProvider = a.MustComponent(peermanager.CName).(peermanager.PeerManagerProvider)
	credProvider := a.Component(credentialprovider.CName)
	if credProvider != nil {
		s.credentialProvider = credProvider.(credentialprovider.CredentialProvider)
	} else {
		s.credentialProvider = credentialprovider.NewNoOp()
	}
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
	s.metric, _ = a.Component(metric.CName).(metric.Metric)
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

	lastConfiguration := s.configurationService
	var (
		spaceIsClosed  = &atomic.Bool{}
		spaceIsDeleted = &atomic.Bool{}
	)
	isDeleted, err := st.IsSpaceDeleted()
	if err != nil {
		return nil, err
	}
	spaceIsDeleted.Swap(isDeleted)
	getter := newCommonGetter(st.Id(), s.treeManager, spaceIsClosed)
	syncStatus := syncstatus.NewNoOpSyncStatus()
	// this will work only for clients, not the best solution, but...
	if !lastConfiguration.IsResponsible(st.Id()) {
		// TODO: move it to the client package and add possibility to inject StatusProvider from the client
		syncStatus = syncstatus.NewSyncStatusProvider(st.Id(), syncstatus.DefaultDeps(lastConfiguration, st))
	}
	var builder objecttree.BuildObjectTreeFunc
	if s.config.KeepTreeDataInMemory {
		builder = objecttree.BuildObjectTree
	} else {
		builder = objecttree.BuildEmptyDataObjectTree
	}

	peerManager, err := s.peermanagerProvider.NewPeerManager(ctx, id)
	if err != nil {
		return nil, err
	}

	headSync := headsync.NewHeadSync(id, spaceIsDeleted, s.config.SyncPeriod, lastConfiguration, st, peerManager, getter, syncStatus, s.credentialProvider, log)
	objectSync := objectsync.NewObjectSync(id, spaceIsDeleted, lastConfiguration, peerManager, getter, st)
	sp := &space{
		id:            id,
		objectSync:    objectSync,
		headSync:      headSync,
		syncStatus:    syncStatus,
		treeManager:   getter,
		account:       s.account,
		configuration: lastConfiguration,
		peerManager:   peerManager,
		storage:       st,
		treesUsed:     &atomic.Int32{},
		treeBuilder:   builder,
		isClosed:      spaceIsClosed,
		isDeleted:     spaceIsDeleted,
		metric:        s.metric,
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

	cl := spacesyncproto.NewDRPCSpaceSyncClient(p)
	res, err := cl.SpacePull(ctx, &spacesyncproto.SpacePullRequest{Id: id})
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
