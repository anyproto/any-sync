package commonspace

import (
	"context"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/headsync"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/objectsync"
	"github.com/anyproto/any-sync/commonspace/objecttreebuilder"
	"github.com/anyproto/any-sync/commonspace/settings"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/util/crypto"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SpaceCreatePayload struct {
	// SigningKey is the signing key of the owner
	SigningKey crypto.PrivKey
	// SpaceType is an arbitrary string
	SpaceType string
	// ReadKey is a first symmetric encryption key for a space
	ReadKey []byte
	// ReplicationKey is a key which is to be used to determine the node where the space should be held
	ReplicationKey uint64
	// SpacePayload is an arbitrary payload related to space type
	SpacePayload []byte
	// MasterKey is the master key of the owner
	MasterKey crypto.PrivKey
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

	StoredIds() []string
	DebugAllHeads() []headsync.TreeHeads
	Description() (desc SpaceDescription, err error)

	TreeBuilder() objecttreebuilder.TreeBuilder
	SyncStatus() syncstatus.StatusUpdater
	Storage() spacestorage.SpaceStorage

	DeleteTree(ctx context.Context, id string) (err error)
	SpaceDeleteRawChange(ctx context.Context) (raw *treechangeproto.RawTreeChangeWithId, err error)
	DeleteSpace(ctx context.Context, deleteChange *treechangeproto.RawTreeChangeWithId) (err error)

	HandleMessage(ctx context.Context, msg objectsync.HandleMessage) (err error)
	HandleSyncRequest(ctx context.Context, req *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error)
	HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error)

	TryClose(objectTTL time.Duration) (close bool, err error)
	Close() error
}

type space struct {
	mu     sync.RWMutex
	header *spacesyncproto.RawSpaceHeaderWithId

	state *spacestate.SpaceState
	app   *app.App

	treeBuilder objecttreebuilder.TreeBuilderComponent
	headSync    headsync.HeadSync
	objectSync  objectsync.ObjectSync
	syncStatus  syncstatus.StatusService
	settings    settings.Settings
	storage     spacestorage.SpaceStorage
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

func (s *space) SpaceDeleteRawChange(ctx context.Context) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	return s.settings.SpaceDeleteRawChange(ctx)
}

func (s *space) DeleteSpace(ctx context.Context, deleteChange *treechangeproto.RawTreeChangeWithId) (err error) {
	return s.settings.DeleteSpace(ctx, deleteChange)
}

func (s *space) HandleMessage(ctx context.Context, msg objectsync.HandleMessage) (err error) {
	return s.objectSync.HandleMessage(ctx, msg)
}

func (s *space) HandleSyncRequest(ctx context.Context, req *spacesyncproto.ObjectSyncMessage) (resp *spacesyncproto.ObjectSyncMessage, err error) {
	return s.objectSync.HandleRequest(ctx, req)
}

func (s *space) HandleRangeRequest(ctx context.Context, req *spacesyncproto.HeadSyncRequest) (resp *spacesyncproto.HeadSyncResponse, err error) {
	return s.headSync.HandleRangeRequest(ctx, req)
}

func (s *space) TreeBuilder() objecttreebuilder.TreeBuilder {
	return s.treeBuilder
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
	s.syncStatus = s.app.MustComponent(syncstatus.CName).(syncstatus.StatusService)
	s.settings = s.app.MustComponent(settings.CName).(settings.Settings)
	s.objectSync = s.app.MustComponent(objectsync.CName).(objectsync.ObjectSync)
	s.storage = s.app.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
	s.aclList = s.app.MustComponent(syncacl.CName).(list.AclList)
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
	if time.Now().Sub(s.objectSync.LastUsage()) < objectTTL {
		return false, nil
	}
	locked := s.state.TreesUsed.Load() > 1
	log.With(zap.Int32("trees used", s.state.TreesUsed.Load()), zap.Bool("locked", locked), zap.String("spaceId", s.state.SpaceId)).Debug("space lock status check")
	if locked {
		return false, nil
	}
	return true, s.Close()
}
