package commonspace

import (
	"context"
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/confconnector"
	"github.com/anytypeio/any-sync/commonspace/headsync"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/commonspace/object/treegetter"
	"github.com/anytypeio/any-sync/commonspace/objectsync"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/commonspace/streammanager"
	"github.com/anytypeio/any-sync/commonspace/syncstatus"
	"github.com/anytypeio/any-sync/net/peer"
	"github.com/anytypeio/any-sync/net/pool"
	"github.com/anytypeio/any-sync/nodeconf"
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
	CreateSpace(ctx context.Context, payload SpaceCreatePayload) (string, error)
	NewSpace(ctx context.Context, id string) (sp Space, err error)
	app.Component
}

type spaceService struct {
	config                Config
	account               accountservice.Service
	configurationService  nodeconf.Service
	storageProvider       spacestorage.SpaceStorageProvider
	streamManagerProvider streammanager.StreamManagerProvider
	treeGetter            treegetter.TreeGetter
	pool                  pool.Pool
}

func (s *spaceService) Init(a *app.App) (err error) {
	s.config = a.MustComponent("config").(ConfigGetter).GetSpace()
	s.account = a.MustComponent(accountservice.CName).(accountservice.Service)
	s.storageProvider = a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorageProvider)
	s.configurationService = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	s.treeGetter = a.MustComponent(treegetter.CName).(treegetter.TreeGetter)
	s.streamManagerProvider = a.MustComponent(streammanager.CName).(streammanager.StreamManagerProvider)
	s.pool = a.MustComponent(pool.CName).(pool.Pool)
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
	store, err := s.storageProvider.CreateSpaceStorage(storageCreate)
	if err != nil {
		if err == spacestorage.ErrSpaceStorageExists {
			return storageCreate.SpaceHeaderWithId.Id, nil
		}
		return
	}

	return store.Id(), nil
}

func (s *spaceService) DeriveSpace(ctx context.Context, payload SpaceDerivePayload) (id string, err error) {
	storageCreate, err := storagePayloadForSpaceDerive(payload)
	if err != nil {
		return
	}
	store, err := s.storageProvider.CreateSpaceStorage(storageCreate)
	if err != nil {
		if err == spacestorage.ErrSpaceStorageExists {
			return storageCreate.SpaceHeaderWithId.Id, nil
		}
		return
	}

	return store.Id(), nil
}

func (s *spaceService) NewSpace(ctx context.Context, id string) (Space, error) {
	st, err := s.storageProvider.SpaceStorage(id)
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

	lastConfiguration := s.configurationService.GetLast()
	confConnector := confconnector.NewConfConnector(lastConfiguration, s.pool)

	syncStatus := syncstatus.NewNoOpSyncStatus()
	// this will work only for clients, not the best solution, but...
	if !lastConfiguration.IsResponsible(st.Id()) {
		// TODO: move it to the client package and add possibility to inject StatusProvider from the client
		syncStatus = syncstatus.NewSyncStatusProvider(st.Id(), syncstatus.DefaultDeps(lastConfiguration, st))
	}

	// TODO: [che] remove *5
	headSync := headsync.NewHeadSync(id, s.config.SyncPeriod*5, st, confConnector, s.treeGetter, syncStatus, log)

	streamManager, err := s.streamManagerProvider.NewStreamManager(ctx, id)
	if err != nil {
		return nil, err
	}

	objectSync := objectsync.NewObjectSync(streamManager, id)
	sp := &space{
		id:            id,
		objectSync:    objectSync,
		headSync:      headSync,
		syncStatus:    syncStatus,
		cache:         s.treeGetter,
		account:       s.account,
		configuration: lastConfiguration,
		storage:       st,
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
	st, err = s.storageProvider.CreateSpaceStorage(payload)
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
	lastConfiguration := s.configurationService.GetLast()
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
		return
	}

	st, err = s.storageProvider.CreateSpaceStorage(spacestorage.SpaceStorageCreatePayload{
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
