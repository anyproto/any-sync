package synctree

import (
	"context"
	"errors"
	"google.golang.org/protobuf/proto"

	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/synctree/response"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/util/slice"
)

var (
	ErrUnexpectedMessageType  = errors.New("unexpected message type")
	ErrUnexpectedRequestType  = errors.New("unexpected request type")
	ErrUnexpectedResponseType = errors.New("unexpected response type")
)

type syncHandler struct {
	tree       SyncTree
	syncClient SyncClient
	spaceId    string
}

var createResponseProducer = response.NewResponseProducer

func NewSyncHandler(tree SyncTree, syncClient SyncClient, spaceId string) syncdeps.ObjectSyncHandler {
	return &syncHandler{
		tree:       tree,
		syncClient: syncClient,
		spaceId:    spaceId,
	}
}

func (s *syncHandler) HandleHeadUpdate(ctx context.Context, statusUpdater syncstatus.StatusUpdater, headUpdate drpc.Message) (req syncdeps.Request, err error) {
	var objectRequest *objectmessages.Request
	defer func() {
		// we mitigate the problem of a nil value being wrapped in an interface
		if err == nil && objectRequest != nil {
			req = objectRequest
		}
	}()
	update, ok := headUpdate.(*objectmessages.HeadUpdate)
	if !ok {
		return nil, ErrUnexpectedResponseType
	}
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	treeSyncMsg := &treechangeproto.TreeSyncMessage{}
	err = treeSyncMsg.UnmarshalVT(update.Bytes)
	if err != nil {
		return nil, err
	}
	objectmessages.FreeHeadUpdate(update)
	if treeSyncMsg.GetContent().GetHeadUpdate() == nil {
		return nil, ErrUnexpectedMessageType
	}
	contentUpdate := treeSyncMsg.GetContent().GetHeadUpdate()
	statusUpdater.HeadsReceive(peerId, update.ObjectId(), contentUpdate.Heads)
	s.tree.Lock()
	defer s.tree.Unlock()
	log.Debug("got head update",
		zap.String("objectId", update.ObjectId()),
		zap.String("peerId", peerId),
		zap.Strings("theirHeads", contentUpdate.Heads),
		zap.Strings("ourHeads", s.tree.Heads()))
	if len(contentUpdate.Changes) == 0 {
		if s.hasHeads(s.tree, contentUpdate.Heads) {
			statusUpdater.HeadsApply(peerId, update.ObjectId(), contentUpdate.Heads, true)
			return nil, nil
		}
		statusUpdater.HeadsApply(peerId, update.ObjectId(), contentUpdate.Heads, false)
		objectRequest, err = s.syncClient.CreateFullSyncRequest(peerId, s.tree)
		return
	}
	rawChangesPayload := objecttree.RawChangesPayload{
		NewHeads:     contentUpdate.Heads,
		RawChanges:   contentUpdate.Changes,
		SnapshotPath: contentUpdate.SnapshotPath,
	}
	res, err := s.tree.AddRawChangesFromPeer(ctx, peerId, rawChangesPayload)
	if err != nil {
		return nil, err
	}
	if !slice.UnsortedEquals(res.Heads, contentUpdate.Heads) {
		objectRequest, err = s.syncClient.CreateFullSyncRequest(peerId, s.tree)
		return
	}
	return nil, nil
}

func (s *syncHandler) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, updater syncdeps.QueueSizeUpdater, send func(resp proto.Message) error) (syncdeps.Request, error) {
	req, ok := rq.(*objectmessages.Request)
	if !ok {
		return nil, ErrUnexpectedRequestType
	}
	treeSyncMsg := &treechangeproto.TreeSyncMessage{}
	err := treeSyncMsg.UnmarshalVT(req.Bytes)
	if err != nil {
		return nil, err
	}
	request := treeSyncMsg.GetContent().GetFullSyncRequest()
	if request == nil {
		return nil, ErrUnexpectedRequestType
	}
	s.tree.Lock()
	curHeads := s.tree.Heads()
	log.Debug("got stream request",
		zap.String("objectId", req.ObjectId()),
		zap.String("peerId", rq.PeerId()),
		zap.Strings("theirHeads", request.Heads),
		zap.Strings("ourHeads", curHeads))
	producer, err := createResponseProducer(s.spaceId, s.tree, request.Heads, request.SnapshotPath)
	if err != nil {
		s.tree.Unlock()
		return nil, err
	}
	var returnReq syncdeps.Request
	if slice.UnsortedEquals(curHeads, request.Heads) || slice.ContainsSorted(request.Heads, curHeads) {
		if len(curHeads) != len(request.Heads) {
			returnReq, err = s.syncClient.CreateFullSyncRequest(rq.PeerId(), s.tree)
			if err != nil {
				s.tree.Unlock()
				return nil, err
			}
		}
		resp, err := producer.EmptyResponse()
		s.tree.Unlock()
		if err != nil {
			return nil, err
		}
		protoResp, err := resp.ProtoMessage()
		if err != nil {
			return nil, err
		}
		return returnReq, send(protoResp)
	} else {
		if len(request.Heads) != 0 {
			returnReq, err = s.syncClient.CreateFullSyncRequest(rq.PeerId(), s.tree)
			if err != nil {
				s.tree.Unlock()
				return nil, err
			}
		}
		s.tree.Unlock()
	}
	for {
		batch, err := producer.NewResponse(batchSize)
		if err != nil {
			return nil, err
		}
		if len(batch.Changes) == 0 {
			break
		}
		size := batch.MsgSize()
		updater.UpdateQueueSize(size, syncdeps.MsgTypeSentResponse, true)
		protoBatch, err := batch.ProtoMessage()
		if err != nil {
			updater.UpdateQueueSize(size, syncdeps.MsgTypeSentResponse, false)
			return nil, err
		}
		err = send(protoBatch)
		updater.UpdateQueueSize(size, syncdeps.MsgTypeSentResponse, false)
		if err != nil {
			return nil, err
		}
	}
	return returnReq, nil
}

func (s *syncHandler) HandleResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	rsp, ok := resp.(*response.Response)
	if !ok {
		return ErrUnexpectedResponseType
	}
	if len(rsp.Changes) == 0 {
		return nil
	}
	s.tree.Lock()
	defer s.tree.Unlock()
	rawChangesPayload := objecttree.RawChangesPayload{
		NewHeads:     rsp.Heads,
		RawChanges:   rsp.Changes,
		SnapshotPath: rsp.SnapshotPath,
	}
	_, err := s.tree.AddRawChangesFromPeer(ctx, peerId, rawChangesPayload)
	return err
}

func (s *syncHandler) ResponseCollector() syncdeps.ResponseCollector {
	return newResponseCollector(s)
}

func (s *syncHandler) hasHeads(ot objecttree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(ot.Heads(), heads) || ot.HasChanges(heads...)
}
