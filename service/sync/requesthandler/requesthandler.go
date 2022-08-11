package requesthandler

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/treepb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/treecache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/syncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"go.uber.org/zap"
)

type requestHandler struct {
	treeCache      treecache.Service
	account        account.Service
	messageService MessageSender
}

var log = logger.NewNamed("requesthandler")

var ErrIncorrectDocType = errors.New("incorrec doc type")

func New() app.Component {
	return &requestHandler{}
}

type RequestHandler interface {
	HandleSyncMessage(ctx context.Context, senderId string, request *syncproto.Sync) (err error)
}

type MessageSender interface {
	SendMessageAsync(peerId string, msg *syncproto.Sync) error
	SendToSpaceAsync(spaceId string, msg *syncproto.Sync) error
}

const CName = "SyncRequestHandler"

func (r *requestHandler) Init(ctx context.Context, a *app.App) (err error) {
	r.treeCache = a.MustComponent(treecache.CName).(treecache.Service)
	r.account = a.MustComponent(account.CName).(account.Service)
	r.messageService = a.MustComponent("MessageService").(MessageSender)
	return nil
}

func (r *requestHandler) Name() (name string) {
	return CName
}

func (r *requestHandler) Run(ctx context.Context) (err error) {
	return nil
}

func (r *requestHandler) Close(ctx context.Context) (err error) {
	return nil
}

func (r *requestHandler) HandleSyncMessage(ctx context.Context, senderId string, content *syncproto.Sync) error {
	msg := content.GetMessage()
	switch {
	case msg.GetFullSyncRequest() != nil:
		return r.HandleFullSyncRequest(ctx, senderId, msg.GetFullSyncRequest())
	case msg.GetFullSyncResponse() != nil:
		return r.HandleFullSyncResponse(ctx, senderId, msg.GetFullSyncResponse())
	case msg.GetHeadUpdate() != nil:
		return r.HandleHeadUpdate(ctx, senderId, msg.GetHeadUpdate())
	}
	return nil
}

func (r *requestHandler) HandleHeadUpdate(ctx context.Context, senderId string, update *syncproto.SyncHeadUpdate) (err error) {
	var (
		fullRequest  *syncproto.SyncFullRequest
		snapshotPath []string
		result       tree.AddResult
	)
	log.With(zap.String("peerId", senderId), zap.String("treeId", update.TreeId)).
		Debug("processing head update")

	updateACLTree := func() {
		err = r.treeCache.Do(ctx, update.TreeId, func(obj interface{}) error {
			t := obj.(tree.ACLTree)
			t.Lock()
			defer t.Unlock()
			// TODO: check if we already have those changes
			result, err = t.AddRawChanges(ctx, update.Changes...)
			if err != nil {
				return err
			}
			log.With(zap.Strings("update heads", update.Heads), zap.Strings("tree heads", t.Heads())).
				Debug("comparing heads after head update")
			shouldFullSync := !slice.UnsortedEquals(update.Heads, t.Heads())
			snapshotPath = t.SnapshotPath()
			if shouldFullSync {
				fullRequest, err = r.prepareFullSyncRequest(update.TreeId, update.TreeHeader, update.SnapshotPath, t)
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	updateDocTree := func() {
		err = r.treeCache.Do(ctx, update.TreeId, func(obj interface{}) error {
			docTree := obj.(tree.DocTree)
			docTree.Lock()
			defer docTree.Unlock()

			return r.treeCache.Do(ctx, update.TreeId, func(obj interface{}) error {
				aclTree := obj.(tree.ACLTree)
				aclTree.RLock()
				defer aclTree.RUnlock()
				// TODO: check if we already have those changes
				result, err = docTree.AddRawChanges(ctx, aclTree, update.Changes...)
				if err != nil {
					return err
				}
				log.With(zap.Strings("update heads", update.Heads), zap.Strings("tree heads", docTree.Heads())).
					Debug("comparing heads after head update")
				shouldFullSync := !slice.UnsortedEquals(update.Heads, docTree.Heads())
				snapshotPath = docTree.SnapshotPath()
				if shouldFullSync {
					fullRequest, err = r.prepareFullSyncRequest(update.TreeId, update.TreeHeader, update.SnapshotPath, docTree)
					if err != nil {
						return err
					}
				}
				return nil
			})
		})
	}

	switch update.TreeHeader.Type {
	case treepb.TreeHeader_ACLTree:
		updateACLTree()
	case treepb.TreeHeader_DocTree:
		updateDocTree()
	default:
		return ErrIncorrectDocType
	}

	// if there are no such tree
	if err == treestorage.ErrUnknownTreeId {
		// TODO: maybe we can optimize this by sending the header and stuff right away, so when the tree is created we are able to add it on first request
		fullRequest = &syncproto.SyncFullRequest{
			TreeId:     update.TreeId,
			TreeHeader: update.TreeHeader,
		}
	}
	// if we have incompatible heads, or we haven't seen the tree at all
	if fullRequest != nil {
		return r.messageService.SendMessageAsync(senderId, syncproto.WrapFullRequest(fullRequest))
	}
	// if error or nothing has changed
	if err != nil || len(result.Added) == 0 {
		return err
	}
	// otherwise sending heads update message
	newUpdate := &syncproto.SyncHeadUpdate{
		Heads:        result.Heads,
		Changes:      result.Added,
		SnapshotPath: snapshotPath,
		TreeId:       update.TreeId,
		TreeHeader:   update.TreeHeader,
	}
	return r.messageService.SendToSpaceAsync("", syncproto.WrapHeadUpdate(newUpdate))
}

func (r *requestHandler) HandleFullSyncRequest(ctx context.Context, senderId string, request *syncproto.SyncFullRequest) (err error) {
	var (
		fullResponse *syncproto.SyncFullResponse
		snapshotPath []string
		result       tree.AddResult
	)
	log.With(zap.String("peerId", senderId), zap.String("treeId", request.TreeId)).
		Debug("processing full sync request")

	requestACLTree := func() {
		err = r.treeCache.Do(ctx, request.TreeId, func(obj interface{}) error {
			t := obj.(tree.ACLTree)
			t.Lock()
			defer t.Unlock()

			// TODO: check if we already have those changes
			// if we have non-empty request
			if len(request.Heads) != 0 {
				result, err = t.AddRawChanges(ctx, request.Changes...)
				if err != nil {
					return err
				}
			}
			snapshotPath = t.SnapshotPath()
			fullResponse, err = r.prepareFullSyncResponse(request.TreeId, request.SnapshotPath, request.Changes, t)
			if err != nil {
				return err
			}
			return nil
		})
	}

	requestDocTree := func() {
		err = r.treeCache.Do(ctx, request.TreeId, func(obj interface{}) error {
			docTree := obj.(tree.DocTree)
			docTree.Lock()
			defer docTree.Unlock()

			return r.treeCache.Do(ctx, request.TreeId, func(obj interface{}) error {
				aclTree := obj.(tree.ACLTree)
				aclTree.RLock()
				defer aclTree.RUnlock()
				// TODO: check if we already have those changes
				// if we have non-empty request
				if len(request.Heads) != 0 {
					result, err = docTree.AddRawChanges(ctx, aclTree, request.Changes...)
					if err != nil {
						return err
					}
				}
				snapshotPath = docTree.SnapshotPath()
				fullResponse, err = r.prepareFullSyncResponse(request.TreeId, request.SnapshotPath, request.Changes, docTree)
				if err != nil {
					return err
				}
				return nil
			})
		})
	}

	switch request.TreeHeader.Type {
	case treepb.TreeHeader_ACLTree:
		requestACLTree()
	case treepb.TreeHeader_DocTree:
		requestDocTree()
	default:
		return ErrIncorrectDocType
	}

	if err != nil {
		return err
	}
	err = r.messageService.SendMessageAsync(senderId, syncproto.WrapFullResponse(fullResponse))
	// if error or nothing has changed
	if err != nil || len(result.Added) == 0 {
		return err
	}

	// otherwise sending heads update message
	newUpdate := &syncproto.SyncHeadUpdate{
		Heads:        result.Heads,
		Changes:      result.Added,
		SnapshotPath: snapshotPath,
		TreeId:       request.TreeId,
		TreeHeader:   request.TreeHeader,
	}
	return r.messageService.SendToSpaceAsync("", syncproto.WrapHeadUpdate(newUpdate))
}

func (r *requestHandler) HandleFullSyncResponse(ctx context.Context, senderId string, response *syncproto.SyncFullResponse) (err error) {
	var (
		snapshotPath []string
		result       tree.AddResult
	)
	log.With(zap.String("peerId", senderId), zap.String("treeId", response.TreeId)).
		Debug("processing full sync response")

	responseACLTree := func() {
		err = r.treeCache.Do(ctx, response.TreeId, func(obj interface{}) error {
			t := obj.(tree.ACLTree)
			t.Lock()
			defer t.Unlock()
			// TODO: check if we already have those changes
			result, err = t.AddRawChanges(ctx, response.Changes...)
			if err != nil {
				return err
			}
			snapshotPath = t.SnapshotPath()
			return nil
		})
	}

	responseDocTree := func() {
		err = r.treeCache.Do(ctx, response.TreeId, func(obj interface{}) error {
			docTree := obj.(tree.DocTree)
			docTree.Lock()
			defer docTree.Unlock()

			return r.treeCache.Do(ctx, response.TreeId, func(obj interface{}) error {
				aclTree := obj.(tree.ACLTree)
				aclTree.RLock()
				defer aclTree.RUnlock()
				// TODO: check if we already have those changes
				result, err = docTree.AddRawChanges(ctx, aclTree, response.Changes...)
				if err != nil {
					return err
				}
				snapshotPath = docTree.SnapshotPath()
				return nil
			})
		})
	}

	switch response.TreeHeader.Type {
	case treepb.TreeHeader_ACLTree:
		responseACLTree()
	case treepb.TreeHeader_DocTree:
		responseDocTree()
	default:
		return ErrIncorrectDocType
	}

	// if error or nothing has changed
	if (err != nil || len(result.Added) == 0) && err != treestorage.ErrUnknownTreeId {
		return err
	}
	// if we have a new tree
	if err == treestorage.ErrUnknownTreeId {
		err = r.createTree(ctx, response)
		if err != nil {
			return err
		}
		result = tree.AddResult{
			OldHeads: []string{},
			Heads:    response.Heads,
			Added:    response.Changes,
		}
	}
	// sending heads update message
	newUpdate := &syncproto.SyncHeadUpdate{
		Heads:        result.Heads,
		Changes:      result.Added,
		SnapshotPath: snapshotPath,
		TreeId:       response.TreeId,
	}
	return r.messageService.SendToSpaceAsync("", syncproto.WrapHeadUpdate(newUpdate))
}

func (r *requestHandler) prepareFullSyncRequest(treeId string, header *treepb.TreeHeader, theirPath []string, t tree.CommonTree) (*syncproto.SyncFullRequest, error) {
	ourChanges, err := t.ChangesAfterCommonSnapshot(theirPath)
	if err != nil {
		return nil, err
	}
	return &syncproto.SyncFullRequest{
		Heads:        t.Heads(),
		Changes:      ourChanges,
		TreeId:       treeId,
		SnapshotPath: t.SnapshotPath(),
		TreeHeader:   header,
	}, nil
}

func (r *requestHandler) prepareFullSyncResponse(
	treeId string,
	theirPath []string,
	theirChanges []*aclpb.RawChange,
	t tree.CommonTree) (*syncproto.SyncFullResponse, error) {
	// TODO: we can probably use the common snapshot calculated on the request step from previous peer
	ourChanges, err := t.ChangesAfterCommonSnapshot(theirPath)
	if err != nil {
		return nil, err
	}
	theirMap := make(map[string]struct{})
	for _, ch := range theirChanges {
		theirMap[ch.Id] = struct{}{}
	}

	// filtering our changes, so we will not send the same changes back
	var final []*aclpb.RawChange
	for _, ch := range ourChanges {
		if _, exists := theirMap[ch.Id]; !exists {
			final = append(final, ch)
		}
	}
	log.With(zap.Int("len(changes)", len(final)), zap.String("id", treeId)).
		Debug("preparing changes for tree")

	return &syncproto.SyncFullResponse{
		Heads:        t.Heads(),
		Changes:      final,
		TreeId:       treeId,
		SnapshotPath: t.SnapshotPath(),
		TreeHeader:   t.Header(),
	}, nil
}

func (r *requestHandler) createTree(ctx context.Context, response *syncproto.SyncFullResponse) error {
	return r.treeCache.Add(
		ctx,
		response.TreeId,
		response.TreeHeader,
		response.Changes,
		func(obj interface{}) error {
			return nil
		})
}
