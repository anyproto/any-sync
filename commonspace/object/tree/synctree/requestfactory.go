package synctree

import (
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
)

const batchSize = 1024 * 1024

type RequestFactory interface {
	CreateHeadUpdate(t objecttree.ObjectTree, ignoredPeer string, added []*treechangeproto.RawTreeChangeWithId) (headUpdate *objectmessages.HeadUpdate)
	CreateNewTreeRequest(peerId, objectId string) *objectmessages.Request
	CreateFullSyncRequest(peerId string, t objecttree.ObjectTree) *objectmessages.Request
	CreateResponseProducer(t objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (ResponseProducer, error)
}

func NewRequestFactory(spaceId string) RequestFactory {
	return &requestFactory{spaceId}
}

type requestFactory struct {
	spaceId string
}

func (r *requestFactory) CreateHeadUpdate(t objecttree.ObjectTree, ignoredPeer string, added []*treechangeproto.RawTreeChangeWithId) (headUpdate *objectmessages.HeadUpdate) {
	broadcastOpts := BroadcastOptions{}
	if ignoredPeer != "" {
		broadcastOpts.EmptyPeers = []string{ignoredPeer}
	}
	return &objectmessages.HeadUpdate{
		Meta: objectmessages.ObjectMeta{
			ObjectId: t.Id(),
			SpaceId:  r.spaceId,
		},
		Update: InnerHeadUpdate{
			opts:         broadcastOpts,
			heads:        t.Heads(),
			changes:      added,
			snapshotPath: t.SnapshotPath(),
			root:         t.Header(),
		},
	}
}

func (r *requestFactory) CreateNewTreeRequest(peerId, objectId string) *objectmessages.Request {
	return NewRequest(peerId, r.spaceId, objectId, nil, nil, nil)
}

func (r *requestFactory) CreateFullSyncRequest(peerId string, t objecttree.ObjectTree) *objectmessages.Request {
	return NewRequest(peerId, r.spaceId, t.Id(), t.Heads(), t.SnapshotPath(), t.Header())
}

func (r *requestFactory) CreateResponseProducer(t objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (ResponseProducer, error) {
	return newResponseProducer(r.spaceId, t, theirHeads, theirSnapshotPath)
}
