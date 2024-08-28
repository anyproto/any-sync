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
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
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
	"github.com/anyproto/any-sync/net/streampool"
	"github.com/anyproto/any-sync/util/crypto"
)

type SpaceCreatePayload struct {
	// SigningKey is the signing key of the owner
	SigningKey crypto.PrivKey
	// SpaceType is an arbitrary string
	SpaceType string
	// ReplicationKey is a key which is to be used to determine the node where the space should be held
	ReplicationKey uint64
	// SpacePayload is an arbitrary payload related to space type
	SpacePayload []byte
	// MasterKey is the master key of the owner
	MasterKey crypto.PrivKey
	// ReadKey is the first read key of space
	ReadKey crypto.SymKey
	// MetadataKey is the first metadata key of space
	MetadataKey crypto.PrivKey
	// Metadata is the metadata of the owner
	Metadata []byte
}

type SpaceDerivePayload struct {
	SigningKey   crypto.PrivKey
	MasterKey    crypto.PrivKey
	SpaceType    string
	SpacePayload []byte
}

type SpaceDescription struct {
	SpaceHeader          *spacesyncproto.RawSpaceHeaderWithId
	AclId                string
	AclPayload           []byte
	SpaceSettingsId      string
	SpaceSettingsPayload []byte
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
	Description() (desc SpaceDescription, err error)

	TreeBuilder() objecttreebuilder.TreeBuilder
	TreeSyncer() treesyncer.TreeSyncer
	AclClient() aclclient.AclSpaceClient
	SyncStatus() syncstatus.StatusUpdater
	Storage() spacestorage.SpaceStorage

	DeleteTree(ctx context.Context, id string) (err error)
	GetNodePeers(ctx context.Context) (peer []peer.Peer, err error)

	HandleDeprecatedObjectSyncRequest(ctx context.Context, req *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error)
	HandleStreamSyncRequest(ctx context.Context, req *spacesyncproto.ObjectSyncMessage, stream drpc.Stream) (err error)
	HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error)
	HandleMessage(ctx context.Context, msg *objectmessages.HeadUpdate) (err error)

	TryClose(objectTTL time.Duration) (close bool, err error)
	Close() error
}

type space struct {
	mu     sync.RWMutex
	header *spacesyncproto.RawSpaceHeaderWithId

	state *spacestate.SpaceState
	app   *app.App

	treeBuilder objecttreebuilder.TreeBuilderComponent
	treeSyncer  treesyncer.TreeSyncer
	peerManager peermanager.PeerManager
	headSync    headsync.HeadSync
	syncService syncservice.SyncService
	streamPool  streampool.StreamPool
	syncStatus  syncstatus.StatusUpdater
	settings    settings.Settings
	storage     spacestorage.SpaceStorage
	aclClient   aclclient.AclSpaceClient
	aclList     list.AclList
}

func (s *space) Description() (desc SpaceDescription, err error) {
	root := s.aclList.Root()
	settingsStorage, err := s.storage.TreeStorage(s.storage.SpaceSettingsId())
	if err != nil {
		return
	}
	settingsRoot, err := settingsStorage.Root()
	if err != nil {
		return
	}

	desc = SpaceDescription{
		SpaceHeader:          s.header,
		AclId:                root.Id,
		AclPayload:           root.Payload,
		SpaceSettingsId:      settingsRoot.Id,
		SpaceSettingsPayload: settingsRoot.RawChange,
	}
	return
}

func (s *space) StoredIds() []string {
	return s.headSync.ExternalIds()
}

func (s *space) DebugAllHeads() []headsync.TreeHeads {
	return s.headSync.DebugAllHeads()
}

func (s *space) DeleteTree(ctx context.Context, id string) (err error) {
	return s.settings.DeleteTree(ctx, id)
}

func (s *space) HandleMessage(peerCtx context.Context, msg *objectmessages.HeadUpdate) (err error) {
	return s.syncService.HandleMessage(peerCtx, msg)
}

func (s *space) HandleStreamSyncRequest(ctx context.Context, req *spacesyncproto.ObjectSyncMessage, stream drpc.Stream) (err error) {
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return
	}
	objSyncReq := objectmessages.NewByteRequest(peerId, req.SpaceId, req.ObjectId, req.Payload)
	return s.syncService.HandleStreamRequest(ctx, objSyncReq, stream)
}

func (s *space) HandleDeprecatedObjectSyncRequest(ctx context.Context, req *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error) {
	return s.syncService.HandleDeprecatedObjectSync(ctx, req)
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
	s.streamPool = s.app.MustComponent(streampool.CName).(streampool.StreamPool)
	s.treeSyncer = s.app.MustComponent(treesyncer.CName).(treesyncer.TreeSyncer)
	s.aclClient = s.app.MustComponent(aclclient.CName).(aclclient.AclSpaceClient)
	s.header, err = s.storage.SpaceHeader()
	return
}

func (s *space) SyncStatus() syncstatus.StatusUpdater {
	return s.syncStatus
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
	log.With(zap.Int32("trees used", s.state.TreesUsed.Load()), zap.Bool("locked", locked), zap.String("spaceId", s.state.SpaceId)).Debug("space lock status check")
	if locked {
		return false, nil
	}
	return true, s.Close()
}
