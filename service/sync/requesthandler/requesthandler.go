package requesthandler

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/list"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"
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
		return r.HandleFullSyncRequest(ctx, senderId, msg.GetFullSyncRequest(), content.GetTreeHeader(), content.GetTreeId())
	case msg.GetFullSyncResponse() != nil:
		return r.HandleFullSyncResponse(ctx, senderId, msg.GetFullSyncResponse(), content.GetTreeHeader(), content.GetTreeId())
	case msg.GetHeadUpdate() != nil:
		return r.HandleHeadUpdate(ctx, senderId, msg.GetHeadUpdate(), content.GetTreeHeader(), content.GetTreeId())
	case msg.GetAclList() != nil:
		return r.HandleACLList(ctx, senderId, msg.GetAclList(), content.GetTreeHeader(), content.GetTreeId())
	}
	return nil
}

func (r *requestHandler) HandleHeadUpdate(
	ctx context.Context,
	senderId string,
	update *syncproto.SyncHeadUpdate,
	header *aclpb.Header,
	treeId string) (err error) {

	var (
		fullRequest  *syncproto.SyncFullRequest
		snapshotPath []string
		result       tree.AddResult
	)
	log.With(zap.String("peerId", senderId), zap.String("treeId", treeId)).
		Debug("processing head update")

	err = r.treeCache.Do(ctx, treeId, func(obj interface{}) error {
		docTree := obj.(tree.ObjectTree)
		docTree.Lock()
		defer docTree.Unlock()

		if slice.UnsortedEquals(update.Heads, docTree.Heads()) {
			return nil
		}

		return r.treeCache.Do(ctx, docTree.Header().AclListId, func(obj interface{}) error {
			aclTree := obj.(list.ACLList)
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
				fullRequest, err = r.prepareFullSyncRequest(update.SnapshotPath, docTree)
				if err != nil {
					return err
				}
			}
			return nil
		})
	})

	// if there are no such tree
	if err == storage.ErrUnknownTreeId {
		// TODO: maybe we can optimize this by sending the header and stuff right away, so when the tree is created we are able to add it on first request
		fullRequest = &syncproto.SyncFullRequest{}
	}
	// if we have incompatible heads, or we haven't seen the tree at all
	if fullRequest != nil {
		return r.messageService.SendMessageAsync(senderId, syncproto.WrapFullRequest(fullRequest, header, treeId))
	}
	// if error or nothing has changed
	if err != nil || len(result.Added) == 0 {
		return err
	}
	log.Info("res", zap.Int("result added", len(result.Added)))
	// otherwise sending heads update message
	newUpdate := &syncproto.SyncHeadUpdate{
		Heads:        result.Heads,
		Changes:      result.Added,
		SnapshotPath: snapshotPath,
	}
	return r.messageService.SendToSpaceAsync("", syncproto.WrapHeadUpdate(newUpdate, header, treeId))
}

func (r *requestHandler) HandleFullSyncRequest(
	ctx context.Context,
	senderId string,
	request *syncproto.SyncFullRequest,
	header *aclpb.Header,
	treeId string) (err error) {

	var (
		fullResponse *syncproto.SyncFullResponse
		snapshotPath []string
		result       tree.AddResult
	)
	log.With(zap.String("peerId", senderId), zap.String("treeId", treeId)).
		Debug("processing full sync request")

	log.Info("getting doc tree from treeCache", zap.String("treeId", treeId))
	err = r.treeCache.Do(ctx, treeId, func(obj interface{}) error {
		docTree := obj.(tree.ObjectTree)
		docTree.Lock()
		defer docTree.Unlock()

		//if slice.UnsortedEquals(request.Heads, docTree.Heads()) {
		//	return nil
		//}
		log.Info("getting tree from treeCache", zap.String("aclId", docTree.Header().AclListId))
		return r.treeCache.Do(ctx, docTree.Header().AclListId, func(obj interface{}) error {
			aclTree := obj.(list.ACLList)
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
			fullResponse, err = r.prepareFullSyncResponse(treeId, request.SnapshotPath, request.Changes, docTree)
			if err != nil {
				return err
			}
			return nil
		})
	})

	if err != nil {
		return err
	}
	err = r.messageService.SendMessageAsync(senderId, syncproto.WrapFullResponse(fullResponse, header, treeId))
	// if error or nothing has changed
	if err != nil || len(result.Added) == 0 {
		return err
	}

	// otherwise sending heads update message
	newUpdate := &syncproto.SyncHeadUpdate{
		Heads:        result.Heads,
		Changes:      result.Added,
		SnapshotPath: snapshotPath,
	}
	return r.messageService.SendToSpaceAsync("", syncproto.WrapHeadUpdate(newUpdate, header, treeId))
}

func (r *requestHandler) HandleFullSyncResponse(
	ctx context.Context,
	senderId string,
	response *syncproto.SyncFullResponse,
	header *aclpb.Header,
	treeId string) (err error) {

	var (
		snapshotPath []string
		result       tree.AddResult
	)
	log.With(zap.String("peerId", senderId), zap.String("treeId", treeId)).
		Debug("processing full sync response")

	err = r.treeCache.Do(ctx, treeId, func(obj interface{}) error {
		docTree := obj.(tree.ObjectTree)
		docTree.Lock()
		defer docTree.Unlock()

		if slice.UnsortedEquals(response.Heads, docTree.Heads()) {
			return nil
		}

		return r.treeCache.Do(ctx, docTree.Header().AclListId, func(obj interface{}) error {
			aclTree := obj.(list.ACLList)
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

	// if error or nothing has changed
	if (err != nil || len(result.Added) == 0) && err != storage.ErrUnknownTreeId {
		return err
	}
	// if we have a new tree
	if err == storage.ErrUnknownTreeId {
		err = r.createTree(ctx, response, header, treeId)
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
	}
	return r.messageService.SendToSpaceAsync("", syncproto.WrapHeadUpdate(newUpdate, header, treeId))
}

func (r *requestHandler) HandleACLList(
	ctx context.Context,
	senderId string,
	req *syncproto.SyncACLList,
	header *aclpb.Header,
	id string) (err error) {

	err = r.treeCache.Do(ctx, id, func(obj interface{}) error {
		return nil
	})
	// do nothing if already added
	if err == nil {
		return nil
	}
	// if not found then add to storage
	if err == storage.ErrUnknownTreeId {
		return r.createACLList(ctx, req, header, id)
	}
	return err
}

func (r *requestHandler) prepareFullSyncRequest(theirPath []string, t tree.ObjectTree) (*syncproto.SyncFullRequest, error) {
	ourChanges, err := t.ChangesAfterCommonSnapshot(theirPath)
	if err != nil {
		return nil, err
	}
	return &syncproto.SyncFullRequest{
		Heads:        t.Heads(),
		Changes:      ourChanges,
		SnapshotPath: t.SnapshotPath(),
	}, nil
}

func (r *requestHandler) prepareFullSyncResponse(
	treeId string,
	theirPath []string,
	theirChanges []*aclpb.RawChange,
	t tree.ObjectTree) (*syncproto.SyncFullResponse, error) {
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
		SnapshotPath: t.SnapshotPath(),
	}, nil
}

func (r *requestHandler) createTree(
	ctx context.Context,
	response *syncproto.SyncFullResponse,
	header *aclpb.Header,
	treeId string) error {

	return r.treeCache.Add(
		ctx,
		treeId,
		treecache.TreePayload{
			Header:  header,
			Changes: response.Changes,
		})
}

func (r *requestHandler) createACLList(
	ctx context.Context,
	req *syncproto.SyncACLList,
	header *aclpb.Header,
	treeId string) error {

	return r.treeCache.Add(
		ctx,
		treeId,
		treecache.ACLListPayload{
			Header:  header,
			Records: req.Records,
		})
}
