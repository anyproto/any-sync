package commonspace

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/acl/aclclient"
	"github.com/anyproto/any-sync/commonspace/headsync"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/keyvalue/kvinterfaces"
	"github.com/anyproto/any-sync/commonspace/object/treesyncer"
	"github.com/anyproto/any-sync/commonspace/objecttreebuilder"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/settings"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	syncservice "github.com/anyproto/any-sync/commonspace/sync"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/nodeconf"
)

type SpaceDescription struct {
	SpaceHeader          *spacesyncproto.RawSpaceHeaderWithId
	AclId                string
	AclPayload           []byte
	SpaceSettingsId      string
	SpaceSettingsPayload []byte
	AclRecords           []*spacesyncproto.AclRecord
}

func NewSpaceId(id string, repKey uint64) string {
	return strings.Join([]string{id, strconv.FormatUint(repKey, 36)}, ".")
}

type Space interface {
	Id() string
	Init(ctx context.Context) error
	Acl() syncacl.SyncAcl

	StoredIds() []string
	DebugAllHeads() []headsync.TreeHeads
	Description(ctx context.Context) (desc SpaceDescription, err error)

	TreeBuilder() objecttreebuilder.TreeBuilder
	TreeSyncer() treesyncer.TreeSyncer
	AclClient() aclclient.AclSpaceClient
	SyncStatus() syncstatus.StatusUpdater
	Storage() spacestorage.SpaceStorage
	KeyValue() kvinterfaces.KeyValueService

	DeleteTree(ctx context.Context, id string) (err error)
	GetNodePeers(ctx context.Context) (peer []peer.Peer, err error)

	HandleStreamSyncRequest(ctx context.Context, req *spacesyncproto.ObjectSyncMessage, stream drpc.Stream) (err error)
	HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error)
	HandleMessage(ctx context.Context, msg *objectmessages.HeadUpdate) (err error)

	TryClose(objectTTL time.Duration) (close bool, err error)
	Close() error
}

type space struct {
	mu    sync.RWMutex
	state *spacestate.SpaceState
	app   *app.App

	treeBuilder  objecttreebuilder.TreeBuilderComponent
	treeSyncer   treesyncer.TreeSyncer
	peerManager  peermanager.PeerManager
	headSync     headsync.HeadSync
	syncService  syncservice.SyncService
	syncStatus   syncstatus.StatusUpdater
	settings     settings.Settings
	storage      spacestorage.SpaceStorage
	aclClient    aclclient.AclSpaceClient
	keyValue     kvinterfaces.KeyValueService
	aclList      list.AclList
	nodeConf     nodeconf.NodeConf
	creationTime time.Time
}

func (s *space) checkReadAccess(ctx context.Context) error {
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return spacesyncproto.ErrForbidden
	}
	if s.nodeConf != nil && len(s.nodeConf.NodeTypes(peerId)) > 0 {
		return nil
	}
	pubKey, err := peer.CtxPubKey(ctx)
	if err != nil {
		return spacesyncproto.ErrForbidden
	}
	s.aclList.RLock()
	perm := s.aclList.AclState().Permissions(pubKey)
	s.aclList.RUnlock()
	if !perm.CanRead() {
		return spacesyncproto.ErrForbidden
	}
	return nil
}

func (s *space) Description(ctx context.Context) (desc SpaceDescription, err error) {
	if err = s.checkReadAccess(ctx); err != nil {
		return
	}
	state, err := s.storage.StateStorage().GetState(ctx)
	if err != nil {
		return
	}
	s.aclList.RLock()
	defer s.aclList.RUnlock()
	recs, err := s.aclList.RecordsAfter(ctx, "")
	if err != nil {
		return
	}
	aclRecs := make([]*spacesyncproto.AclRecord, 0, len(recs))
	for _, rec := range recs {
		aclRecs = append(aclRecs, &spacesyncproto.AclRecord{
			Id:         rec.Id,
			AclPayload: rec.Payload,
		})
	}
	root := s.aclList.Root()
	settingsStorage, err := s.storage.TreeStorage(ctx, state.SettingsId)
	if err != nil {
		return
	}
	settingsRoot, err := settingsStorage.Root(ctx)
	if err != nil {
		return
	}

	desc = SpaceDescription{
		SpaceHeader: &spacesyncproto.RawSpaceHeaderWithId{
			RawHeader: state.SpaceHeader,
			Id:        state.SpaceId,
		},
		AclId:                root.Id,
		AclPayload:           root.Payload,
		SpaceSettingsId:      settingsRoot.Id,
		SpaceSettingsPayload: settingsRoot.RawChange,
		AclRecords:           aclRecs,
	}
	return
}

func (s *space) StoredIds() []string {
	return s.headSync.ExternalIds()
}

func (s *space) DebugAllHeads() (heads []headsync.TreeHeads) {
	s.storage.HeadStorage().IterateEntries(context.Background(), headstorage.IterOpts{}, func(entry headstorage.HeadsEntry) (bool, error) {
		if entry.CommonSnapshot != "" {
			heads = append(heads, headsync.TreeHeads{
				Id:    entry.Id,
				Heads: entry.Heads,
			})
		}
		return true, nil
	})
	return heads
}

func (s *space) DeleteTree(ctx context.Context, id string) (err error) {
	return s.settings.DeleteTree(ctx, id)
}

func (s *space) HandleMessage(peerCtx context.Context, msg *objectmessages.HeadUpdate) (err error) {
	if err = s.checkReadAccess(peerCtx); err != nil {
		return
	}
	return s.syncService.HandleMessage(peerCtx, msg)
}

func (s *space) HandleStreamSyncRequest(ctx context.Context, req *spacesyncproto.ObjectSyncMessage, stream drpc.Stream) (err error) {
	if err = s.checkReadAccess(ctx); err != nil {
		return
	}
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return
	}
	objSyncReq := objectmessages.NewByteRequest(peerId, req.SpaceId, req.ObjectId, req.Payload)
	return s.syncService.HandleStreamRequest(ctx, objSyncReq, stream)
}

func (s *space) HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error) {
	return s.headSync.HandleRangeRequest(ctx, req)
}

func (s *space) TreeBuilder() objecttreebuilder.TreeBuilder {
	return s.treeBuilder
}

func (s *space) TreeSyncer() treesyncer.TreeSyncer {
	return s.treeSyncer
}

func (s *space) AclClient() aclclient.AclSpaceClient {
	return s.aclClient
}

func (s *space) GetNodePeers(ctx context.Context) (peer []peer.Peer, err error) {
	return s.peerManager.GetNodePeers(ctx)
}

func (s *space) Acl() syncacl.SyncAcl {
	return s.aclList.(syncacl.SyncAcl)
}

func (s *space) Id() string {
	return s.state.SpaceId
}

func (s *space) Init(ctx context.Context) (err error) {
	s.creationTime = time.Now()
	err = s.app.Start(ctx)
	if err != nil {
		return
	}
	s.treeBuilder = s.app.MustComponent(objecttreebuilder.CName).(objecttreebuilder.TreeBuilderComponent)
	s.headSync = s.app.MustComponent(headsync.CName).(headsync.HeadSync)
	s.syncStatus = s.app.MustComponent(syncstatus.CName).(syncstatus.StatusUpdater)
	s.settings = s.app.MustComponent(settings.CName).(settings.Settings)
	s.syncService = s.app.MustComponent(syncservice.CName).(syncservice.SyncService)
	s.storage = s.app.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
	s.peerManager = s.app.MustComponent(peermanager.CName).(peermanager.PeerManager)
	s.aclList = s.app.MustComponent(syncacl.CName).(list.AclList)
	s.treeSyncer = s.app.MustComponent(treesyncer.CName).(treesyncer.TreeSyncer)
	s.aclClient = s.app.MustComponent(aclclient.CName).(aclclient.AclSpaceClient)
	s.keyValue = s.app.MustComponent(kvinterfaces.CName).(kvinterfaces.KeyValueService)
	if nc := s.app.Component(nodeconf.CName); nc != nil {
		s.nodeConf = nc.(nodeconf.NodeConf)
	}
	return
}

func (s *space) SyncStatus() syncstatus.StatusUpdater {
	return s.syncStatus
}

func (s *space) KeyValue() kvinterfaces.KeyValueService {
	return s.keyValue
}

func (s *space) Storage() spacestorage.SpaceStorage {
	return s.storage
}

func (s *space) Close() error {
	if s.state.SpaceIsClosed.Swap(true) {
		log.Warn("call space.Close on closed space", zap.String("id", s.state.SpaceId))
		return nil
	}
	log := log.With(zap.String("spaceId", s.state.SpaceId))
	log.Debug("space is closing")

	err := s.app.Close(context.Background())
	log.Debug("space closed")
	return err
}

func (s *space) TryClose(objectTTL time.Duration) (close bool, err error) {
	locked := s.state.TreesUsed.Load() > 1
	inCacheSecs := int(time.Now().Sub(s.creationTime).Seconds())
	log.With(zap.Int32("trees used", s.state.TreesUsed.Load()), zap.Bool("locked", locked), zap.String("spaceId", s.state.SpaceId), zap.Int("in cache secs", inCacheSecs)).Debug("space lock status check")
	if locked {
		return false, nil
	}
	return true, s.Close()
}
