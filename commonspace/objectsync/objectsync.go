package objectsync

import (
	"context"
	"github.com/anytypeio/any-sync/commonspace/object/tree/synctree"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/gogo/protobuf/proto"
	"sync/atomic"
	"time"

	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/syncobjectgetter"
	"github.com/anytypeio/any-sync/commonspace/objectsync/synchandler"
	"github.com/anytypeio/any-sync/commonspace/peermanager"
	"github.com/anytypeio/any-sync/commonspace/spacestorage"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/nodeconf"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

var log = logger.NewNamed("common.commonspace.objectsync")

type ObjectSync interface {
	LastUsage
	synchandler.SyncHandler
	MessagePool() MessagePool

	Close() (err error)
}

type objectSync struct {
	spaceId string

	messagePool   MessagePool
	syncClient    synctree.SyncClient
	objectGetter  syncobjectgetter.SyncObjectGetter
	configuration nodeconf.Configuration
	spaceStorage  spacestorage.SpaceStorage

	syncCtx        context.Context
	cancelSync     context.CancelFunc
	spaceIsDeleted *atomic.Bool
}

func NewObjectSync(
	spaceId string,
	spaceIsDeleted *atomic.Bool,
	configuration nodeconf.Configuration,
	peerManager peermanager.PeerManager,
	objectGetter syncobjectgetter.SyncObjectGetter,
	storage spacestorage.SpaceStorage) ObjectSync {
	syncCtx, cancel := context.WithCancel(context.Background())
	os := &objectSync{
		objectGetter:   objectGetter,
		spaceStorage:   storage,
		spaceId:        spaceId,
		syncCtx:        syncCtx,
		cancelSync:     cancel,
		spaceIsDeleted: spaceIsDeleted,
		configuration:  configuration,
		syncClient:     synctree.NewSyncClient(spaceId, peerManager, synctree.GetRequestFactory()),
	}
	os.messagePool = newMessagePool(peerManager, os.handleMessage)
	return os
}

func (s *objectSync) Close() (err error) {
	s.cancelSync()
	return
}

func (s *objectSync) LastUsage() time.Time {
	return s.messagePool.LastUsage()
}

func (s *objectSync) HandleMessage(ctx context.Context, senderId string, message *spacesyncproto.ObjectSyncMessage) (err error) {
	return s.messagePool.HandleMessage(ctx, senderId, message)
}

func (s *objectSync) handleMessage(ctx context.Context, senderId string, msg *spacesyncproto.ObjectSyncMessage) (err error) {
	log := log.With(zap.String("objectId", msg.ObjectId), zap.String("replyId", msg.ReplyId))
	if s.spaceIsDeleted.Load() {
		log = log.With(zap.Bool("isDeleted", true))
		// preventing sync with other clients if they are not just syncing the settings tree
		if !slices.Contains(s.configuration.NodeIds(s.spaceId), senderId) && msg.ObjectId != s.spaceStorage.SpaceSettingsId() {
			return spacesyncproto.ErrSpaceIsDeleted
		}
	}
	log.With(zap.String("objectId", msg.ObjectId), zap.String("replyId", msg.ReplyId)).DebugCtx(ctx, "handling message")
	obj, err := s.objectGetter.GetObject(ctx, msg.ObjectId)
	if err != nil {
		respErr := s.sendErrorResponse(ctx, msg, senderId, err)
		if respErr != nil {
			log.Debug("failed to send error response", zap.Error(respErr))
		}
		return
	}
	return obj.HandleMessage(ctx, senderId, msg)
}

func (s *objectSync) MessagePool() MessagePool {
	return s.messagePool
}

func (s *objectSync) sendErrorResponse(ctx context.Context, msg *spacesyncproto.ObjectSyncMessage, senderId string, respErr error) (err error) {
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(msg.Payload, unmarshalled)
	if err != nil {
		return
	}
	resp := treechangeproto.WrapError(respErr, unmarshalled.RootChange)
	return s.syncClient.SendWithReply(ctx, senderId, resp, msg.ReplyId)
}
