package synctree

import (
	"context"
	"errors"

	"github.com/gogo/protobuf/proto"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/util/slice"
)

var (
	ErrUnexpectedMessageType = errors.New("unexpected response type")
	ErrUnexpectedRequestType = errors.New("unexpected request type")
)

type syncHandler struct {
	tree       SyncTree
	syncClient SyncClient
	spaceId    string
}

func NewSyncHandler(tree SyncTree, syncClient SyncClient, spaceId string) syncdeps.ObjectSyncHandler {
	return &syncHandler{
		tree:       tree,
		syncClient: syncClient,
		spaceId:    spaceId,
	}
}

func (s *syncHandler) HandleHeadUpdate(ctx context.Context, headUpdate drpc.Message) (req syncdeps.Request, err error) {
	update, ok := headUpdate.(HeadUpdate)
	if !ok {
		return nil, ErrUnexpectedResponseType
	}
	peerId, err := peer.CtxPeerId(ctx)
	if err != nil {
		return nil, err
	}
	s.tree.Lock()
	defer s.tree.Unlock()
	if len(update.changes) == 0 {
		if s.hasHeads(s.tree, update.heads) {
			return nil, nil
		}
		return s.syncClient.CreateFullSyncRequest(peerId, s.tree), nil
	}
	rawChangesPayload := objecttree.RawChangesPayload{
		NewHeads:   update.heads,
		RawChanges: update.changes,
	}
	res, err := s.tree.AddRawChangesFromPeer(ctx, peerId, rawChangesPayload)
	if err != nil {
		return nil, err
	}
	if !slice.UnsortedEquals(res.Heads, update.heads) {
		return s.syncClient.CreateFullSyncRequest(peerId, s.tree), nil
	}
	return nil, nil
}

func (s *syncHandler) HandleStreamRequest(ctx context.Context, rq syncdeps.Request, send func(resp proto.Message) error) (syncdeps.Request, error) {
	req, ok := rq.(Request)
	if !ok {
		return nil, ErrUnexpectedResponseType
	}
	s.tree.Lock()
	curHeads := s.tree.Heads()
	producer, err := newResponseProducer(s.spaceId, s.tree, req.heads, req.snapshotPath)
	if err != nil {
		s.tree.Unlock()
		return nil, err
	}
	for {
		batch, err := producer.NewResponse(batchSize)
		s.tree.Unlock()
		if err != nil {
			return nil, err
		}
		if len(batch.changes) == 0 {
			break
		}
		protoBatch, err := batch.ProtoMessage()
		if err != nil {
			return nil, err
		}
		err = send(protoBatch)
		if err != nil {
			return nil, err
		}
		s.tree.Lock()
	}
	if !slice.UnsortedEquals(curHeads, req.heads) {
		return s.syncClient.CreateFullSyncRequest(rq.PeerId(), s.tree), nil
	}
	return nil, nil
}

func (s *syncHandler) HandleResponse(ctx context.Context, peerId, objectId string, resp syncdeps.Response) error {
	response, ok := resp.(Response)
	if !ok {
		return ErrUnexpectedResponseType
	}
	s.tree.Lock()
	defer s.tree.Unlock()
	rawChangesPayload := objecttree.RawChangesPayload{
		NewHeads:   response.heads,
		RawChanges: response.changes,
	}
	_, err := s.tree.AddRawChangesFromPeer(ctx, peerId, rawChangesPayload)
	return err
}

func (s *syncHandler) hasHeads(ot objecttree.ObjectTree, heads []string) bool {
	return slice.UnsortedEquals(ot.Heads(), heads) || ot.HasChanges(heads...)
}
