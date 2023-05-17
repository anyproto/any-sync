//go:generate mockgen -destination mock_objectsync/mock_objectsync.go github.com/anytypeio/any-sync/commonspace/objectsync SyncClient
package objectsync

import (
	"context"
	"fmt"
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
	SyncClient() SyncClient

	Close() (err error)
}

type objectSync struct {
	spaceId string

	messagePool   MessagePool
	syncClient    SyncClient
	objectGetter  syncobjectgetter.SyncObjectGetter
	configuration nodeconf.NodeConf
	spaceStorage  spacestorage.SpaceStorage

	syncCtx        context.Context
	cancelSync     context.CancelFunc
	spaceIsDeleted *atomic.Bool
}

func NewObjectSync(
	spaceId string,
	spaceIsDeleted *atomic.Bool,
	configuration nodeconf.NodeConf,
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
	}
	os.messagePool = newMessagePool(peerManager, os.handleMessage)
	os.syncClient = NewSyncClient(spaceId, os.messagePool, NewRequestFactory())
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
	log := log.With(
		zap.String("objectId", msg.ObjectId),
		zap.String("requestId", msg.RequestId),
		zap.String("replyId", msg.ReplyId))
	if s.spaceIsDeleted.Load() {
		log = log.With(zap.Bool("isDeleted", true))
		// preventing sync with other clients if they are not just syncing the settings tree
		if !slices.Contains(s.configuration.NodeIds(s.spaceId), senderId) && msg.ObjectId != s.spaceStorage.SpaceSettingsId() {
			s.unmarshallSendError(ctx, msg, spacesyncproto.ErrUnexpected, senderId, msg.ObjectId)
			return fmt.Errorf("can't perform operation with object, space is deleted")
		}
	}
	log.DebugCtx(ctx, "handling message")
	hasTree, err := s.spaceStorage.HasTree(msg.ObjectId)
	if err != nil {
		s.unmarshallSendError(ctx, msg, spacesyncproto.ErrUnexpected, senderId, msg.ObjectId)
		return fmt.Errorf("falied to execute get operation on storage has tree: %w", err)
	}
	// in this case we will try to get it from remote, unless the sender also sent us the same request :-)
	if !hasTree {
		treeMsg := &treechangeproto.TreeSyncMessage{}
		err = proto.Unmarshal(msg.Payload, treeMsg)
		if err != nil {
			s.sendError(ctx, nil, spacesyncproto.ErrUnexpected, senderId, msg.ObjectId, msg.RequestId)
			return fmt.Errorf("failed to unmarshall tree sync message: %w", err)
		}
		// this means that we don't have the tree locally and therefore can't return it
		if s.isEmptyFullSyncRequest(treeMsg) {
			err = treechangeproto.ErrGetTree
			s.sendError(ctx, nil, treechangeproto.ErrGetTree, senderId, msg.ObjectId, msg.RequestId)
			return fmt.Errorf("failed to get tree from storage on full sync: %w", err)
		}
	}
	obj, err := s.objectGetter.GetObject(ctx, msg.ObjectId)
	if err != nil {
		// TODO: write tests for object sync https://linear.app/anytype/issue/GO-1299/write-tests-for-commonspaceobjectsync
		s.unmarshallSendError(ctx, msg, spacesyncproto.ErrUnexpected, senderId, msg.ObjectId)
		return fmt.Errorf("failed to get object from cache: %w", err)
	}
	// TODO: unmarshall earlier
	err = obj.HandleMessage(ctx, senderId, msg)
	if err != nil {
		s.unmarshallSendError(ctx, msg, spacesyncproto.ErrUnexpected, senderId, msg.ObjectId)
		return fmt.Errorf("failed to handle message: %w", err)
	}
	return
}

func (s *objectSync) SyncClient() SyncClient {
	return s.syncClient
}

func (s *objectSync) unmarshallSendError(ctx context.Context, msg *spacesyncproto.ObjectSyncMessage, respErr error, senderId, objectId string) {
	unmarshalled := &treechangeproto.TreeSyncMessage{}
	err := proto.Unmarshal(msg.Payload, unmarshalled)
	if err != nil {
		return
	}
	s.sendError(ctx, unmarshalled.RootChange, respErr, senderId, objectId, msg.RequestId)
}

func (s *objectSync) sendError(ctx context.Context, root *treechangeproto.RawTreeChangeWithId, respErr error, senderId, objectId, replyId string) {
	// we don't send errors if have no reply id, this can lead to bugs and also nobody needs this error
	if replyId == "" {
		return
	}
	resp := treechangeproto.WrapError(respErr, root)
	if err := s.syncClient.SendWithReply(ctx, senderId, objectId, resp, replyId); err != nil {
		log.InfoCtx(ctx, "failed to send error to client")
	}
}

func (s *objectSync) isEmptyFullSyncRequest(msg *treechangeproto.TreeSyncMessage) bool {
	return msg.GetContent().GetFullSyncRequest() != nil && len(msg.GetContent().GetFullSyncRequest().GetHeads()) == 0
}
