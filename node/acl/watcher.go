package acl

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	"go.uber.org/zap"
	"sync"
)

func newWatcher(spaceId, aclId string, h synchandler.SyncHandler) (w *watcher, err error) {
	w = &watcher{
		aclId:   aclId,
		spaceId: spaceId,
		handler: h,
		ready:   make(chan struct{}),
	}
	if w.logId, err = cidToByte(aclId); err != nil {
		return nil, err
	}
	return
}

type watcher struct {
	spaceId string
	aclId   string
	logId   []byte
	handler synchandler.SyncHandler
	ready   chan struct{}
	isReady sync.Once
	err     error
}

func (w *watcher) AddConsensusRecords(recs []*consensusproto.Record) {
	w.isReady.Do(func() {
		close(w.ready)
	})
	records := make([]*aclrecordproto.RawACLRecordWithId, 0, len(recs))

	for _, rec := range recs {
		recId, err := cidToString(rec.Id)
		if err != nil {
			log.Error("received invalid id from consensus node", zap.Error(err))
			continue
		}
		records = append(records, &aclrecordproto.RawACLRecordWithId{
			Payload: rec.Payload,
			Id:      recId,
		})
	}

	aclReq := &aclrecordproto.ACLSyncMessage{
		Content: &aclrecordproto.ACLSyncContentValue{
			Value: &aclrecordproto.ACLSyncContentValue_AddRecords{
				AddRecords: &aclrecordproto.ACLAddRecords{
					Records: records,
				},
			},
		},
	}
	payload, err := aclReq.Marshal()
	if err != nil {
		log.Error("acl payload marshal error", zap.Error(err))
		return
	}
	req := &spacesyncproto.ObjectSyncMessage{
		SpaceId:  w.spaceId,
		Payload:  payload,
		ObjectId: w.aclId,
	}

	if err = w.handler.HandleMessage(context.TODO(), "", req); err != nil {
		log.Warn("handle message error", zap.Error(err))
	}
}

func (w *watcher) AddConsensusError(err error) {
	w.isReady.Do(func() {
		w.err = err
		close(w.ready)
	})
}

func (w *watcher) Ready(ctx context.Context) (err error) {
	select {
	case <-w.ready:
		return w.err
	case <-ctx.Done():
		return ctx.Err()
	}
}
