package synctree

import (
	"context"
	"errors"
	"fmt"

	"github.com/anyproto/protobuf/proto"
	"go.uber.org/zap"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	response "github.com/anyproto/any-sync/commonspace/object/tree/synctree/response"
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
	update, ok := headUpdate.(*objectmessages.HeadUpdate)
	if !ok {
		return nil, ErrUnexpectedResponseType
	}
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	treeSyncMsg := &treechangeproto.TreeSyncMessage{}
	err = proto.Unmarshal(update.Bytes, treeSyncMsg)
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
	fmt.Println("[x]: receive head update, peer:", update.PeerId(), "object:", update.ObjectId(), "heads:", contentUpdate.Heads, "my heads", s.tree.Heads())
	if len(contentUpdate.Changes) == 0 {
		if s.hasHeads(s.tree, contentUpdate.Heads) {
			fmt.Println("[x]: receive head update, no changes, has heads", update.PeerId(), "object:", update.ObjectId())
			statusUpdater.HeadsApply(peerId, update.ObjectId(), contentUpdate.Heads, true)
			return nil, nil
		}
		fmt.Println("[x]: receive head update, no changes, heads not equal", update.PeerId(), "object:", update.ObjectId())
		statusUpdater.HeadsApply(peerId, update.ObjectId(), contentUpdate.Heads, false)
		return s.syncClient.CreateFullSyncRequest(peerId, s.tree), nil
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
	fmt.Println("[x]: receive head update, after apply", update.PeerId(), "object:", update.ObjectId(), "update heads", contentUpdate.Heads, "result heads", res.Heads)
	if !slice.UnsortedEquals(res.Heads, contentUpdate.Heads) {
		return s.syncClient.CreateFullSyncRequest(peerId, s.tree), nil
	}
	return nil, nil
}

func (s *syncHandler) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, updater syncdeps.QueueSizeUpdater, send func(resp proto.Message) error) (syncdeps.Request, error) {
	req, ok := rq.(*objectmessages.Request)
	if !ok {
		return nil, ErrUnexpectedRequestType
	}
	treeSyncMsg := &treechangeproto.TreeSyncMessage{}
	err := proto.Unmarshal(req.Bytes, treeSyncMsg)
	if err != nil {
		return nil, err
	}
	request := treeSyncMsg.GetContent().GetFullSyncRequest()
	if request == nil {
		return nil, ErrUnexpectedRequestType
	}
	s.tree.Lock()
	curHeads := s.tree.Heads()
	log.Debug("got stream request", zap.String("objectId", req.ObjectId()), zap.String("peerId", rq.PeerId()))
	producer, err := createResponseProducer(s.spaceId, s.tree, request.Heads, request.SnapshotPath)
	if err != nil {
		s.tree.Unlock()
		return nil, err
	}
	var returnReq syncdeps.Request
	if slice.UnsortedEquals(curHeads, request.Heads) || slice.ContainsSorted(request.Heads, curHeads) {
		fmt.Println("[x]: receive stream request, contains heads", rq.PeerId(), "object:", rq.ObjectId(), "update heads", request.Heads, "result heads", s.tree.Heads())
		if len(curHeads) != len(request.Heads) {
			returnReq = s.syncClient.CreateFullSyncRequest(rq.PeerId(), s.tree)
		}
		resp := producer.EmptyResponse()
		s.tree.Unlock()
		protoResp, err := resp.ProtoMessage()
		if err != nil {
			return nil, err
		}
		return returnReq, send(protoResp)
	} else {
		fmt.Println("[x]: receive stream request, not contains heads", rq.PeerId(), "object:", rq.ObjectId(), "update heads", request.Heads, "result heads", s.tree.Heads())
		if len(request.Heads) != 0 {
			returnReq = s.syncClient.CreateFullSyncRequest(rq.PeerId(), s.tree)
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
	fmt.Println("[x]: receive stream response", peerId, "object:", objectId, "update heads", rawChangesPayload.NewHeads, "our heads", s.tree.Heads())
	res, err := s.tree.AddRawChangesFromPeer(ctx, peerId, rawChangesPayload)
	if err != nil {
		return err
	}
	fmt.Println("[x]: apply stream response", peerId, "object:", objectId, "update heads", rawChangesPayload.NewHeads, "applied heads", res.Heads)
	return err
}

func (s *syncHandler) ResponseCollector() syncdeps.ResponseCollector {
	return newResponseCollector(s)
}

func (s *syncHandler) hasHeads(ot objecttree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(ot.Heads(), heads) || ot.HasChanges(heads...)
}
