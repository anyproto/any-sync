//go:generate mockgen -destination mock_response/mock_response.go github.com/anyproto/any-sync/commonspace/object/tree/synctree/response ResponseProducer
package response

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

func NewResponseProducer(spaceId string, tree objecttree.ObjectTree, theirHeads, theirSnapshotPath []string) (ResponseProducer, error) {
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
		Heads:        res.Heads,
		SnapshotPath: res.SnapshotPath,
		Changes:      res.Batch,
		Root:         res.Root,
		SpaceId:      r.spaceId,
		ObjectId:     r.objectId,
	}, nil
}

func (r *responseProducer) EmptyResponse() *Response {
	headsCopy := make([]string, len(r.tree.Heads()))
	copy(headsCopy, r.tree.Heads())
	return &Response{
		Heads:        headsCopy,
		SpaceId:      r.spaceId,
		ObjectId:     r.objectId,
		Root:         r.tree.Header(),
		SnapshotPath: r.tree.SnapshotPath(),
	}
}
