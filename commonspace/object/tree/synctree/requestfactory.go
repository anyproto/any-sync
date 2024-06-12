package synctree

import (
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
)

const batchSize = 1024 * 1024 * 10

type RequestFactory interface {
	CreateHeadUpdate(t objecttree.ObjectTree, ignoredPeer string, added []*treechangeproto.RawTreeChangeWithId) (headUpdate HeadUpdate)
	CreateNewTreeRequest(peerId, objectId string) Request
	CreateFullSyncRequest(peerId string, t objecttree.ObjectTree) Request
	CreateResponseProducer(t objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (ResponseProducer, error)
}

func NewRequestFactory(spaceId string) RequestFactory {
	return &requestFactory{spaceId}
}

type requestFactory struct {
	spaceId string
}

func (r *requestFactory) CreateHeadUpdate(t objecttree.ObjectTree, ignoredPeer string, added []*treechangeproto.RawTreeChangeWithId) (headUpdate HeadUpdate) {
	broadcastOpts := BroadcastOptions{}
	if ignoredPeer != "" {
		broadcastOpts.EmptyPeers = []string{ignoredPeer}
	}
	return HeadUpdate{
		objectId:     t.Id(),
		spaceId:      r.spaceId,
		heads:        t.Heads(),
		changes:      added,
		snapshotPath: t.SnapshotPath(),
		root:         t.Header(),
		opts:         broadcastOpts,
	}
}

func (r *requestFactory) CreateNewTreeRequest(peerId, objectId string) Request {
	return NewRequest(peerId, r.spaceId, objectId, nil, nil, nil)
}

func (r *requestFactory) CreateFullSyncRequest(peerId string, t objecttree.ObjectTree) Request {
	return NewRequest(peerId, r.spaceId, t.Id(), t.Heads(), t.SnapshotPath(), t.Header())
}

func (r *requestFactory) CreateResponseProducer(t objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (ResponseProducer, error) {
	return newResponseProducer(r.spaceId, t, theirHeads, theirSnapshotPath)
}
