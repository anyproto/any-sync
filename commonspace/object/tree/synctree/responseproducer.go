package synctree

import (
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
)

type ResponseProducer interface {
	NewResponse(batchSize int) (*Response, error)
	EmptyResponse() *Response
}

type responseProducer struct {
	iterator objecttree.LoadIterator
	tree     objecttree.ObjectTree
	spaceId  string
	objectId string
}

func newResponseProducer(spaceId string, tree objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (ResponseProducer, error) {
	res, err := tree.ChangesAfterCommonSnapshotLoader(theirSnapshotPath, theirHeads)
	if err != nil {
		return nil, err
	}
	return &responseProducer{
		iterator: res,
		tree:     tree,
		spaceId:  spaceId,
		objectId: tree.Id(),
	}, nil
}

func (r *responseProducer) NewResponse(batchSize int) (*Response, error) {
	res, err := r.iterator.NextBatch(batchSize)
	if err != nil {
		return &Response{}, err
	}
	return &Response{
		heads:        res.Heads,
		snapshotPath: res.SnapshotPath,
		changes:      res.Batch,
		root:         res.Root,
		spaceId:      r.spaceId,
		objectId:     r.objectId,
	}, nil
}

func (r *responseProducer) EmptyResponse() *Response {
	headsCopy := make([]string, len(r.tree.Heads()))
	copy(headsCopy, r.tree.Heads())
	return &Response{
		heads:        headsCopy,
		spaceId:      r.spaceId,
		objectId:     r.objectId,
		root:         r.tree.Header(),
		snapshotPath: r.tree.SnapshotPath(),
	}
}
