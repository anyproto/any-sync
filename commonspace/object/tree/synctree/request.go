package synctree

import (
	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

type Request struct {
	peerId       string
	spaceId      string
	objectId     string
	heads        []string
	snapshotPath []string
	root         *treechangeproto.RawTreeChangeWithId
}

func NewRequest(peerId, spaceId, objectId string, heads []string, snapshotPath []string, root *treechangeproto.RawTreeChangeWithId) Request {
	return Request{
		peerId:       peerId,
		spaceId:      spaceId,
		objectId:     objectId,
		heads:        heads,
		snapshotPath: snapshotPath,
		root:         root,
	}
}

func (r Request) PeerId() string {
	return r.peerId
}

func (r Request) ObjectId() string {
	return r.root.Id
}

func (r Request) Proto() (proto.Message, error) {
	msg := &treechangeproto.TreeFullSyncRequest{
		Heads:        r.heads,
		SnapshotPath: r.snapshotPath,
	}
	req := treechangeproto.WrapFullRequest(msg, r.root)
	return spacesyncproto.MarshallSyncMessage(req, r.spaceId, r.objectId)
}
