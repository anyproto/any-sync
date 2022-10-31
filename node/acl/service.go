package acl

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusclient"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	"time"
)

const CName = "node.acl"

var log = logger.NewNamed(CName)

type Service interface {
	app.Component
}

type service struct {
	consService consensusclient.Service
	account     account.Service
}

func (s *service) Init(a *app.App) (err error) {
	s.consService = a.MustComponent(consensusclient.CName).(consensusclient.Service)
	s.account = a.MustComponent(account.CName).(account.Service)
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateLog(ctx context.Context, aclId string, rec *aclrecordproto.RawACLRecordWithId) (err error) {
	logId, err := cidToByte(aclId)
	if err != nil {
		return
	}
	recId, err := cidToByte(rec.Id)
	if err != nil {
		return
	}
	recPayload, err := rec.Marshal()
	if err != nil {
		return
	}
	return s.consService.AddLog(ctx, &consensusproto.Log{
		Id: logId,
		Records: []*consensusproto.Record{
			{
				Id:          recId,
				Payload:     recPayload,
				CreatedUnix: uint64(time.Now().Unix()),
			},
		},
	})
}

func (s *service) AddRecord(ctx context.Context, aclId string, rec *aclrecordproto.RawACLRecordWithId) (err error) {
	logId, err := cidToByte(aclId)
	if err != nil {
		return
	}
	recId, err := cidToByte(rec.Id)
	if err != nil {
		return
	}

	recPayload, err := rec.Marshal()
	if err != nil {
		return
	}
	return s.consService.AddRecord(ctx, logId, &consensusproto.Record{
		Id:          recId,
		PrevId:      nil, //TODO:
		Payload:     recPayload,
		CreatedUnix: uint64(time.Now().Unix()),
	})
}

func (s *service) Watch(ctx context.Context, spaceId, aclId string, h synchandler.SyncHandler) (err error) {
	w, err := newWatcher(spaceId, aclId, h)
	if err != nil {
		return
	}
	if err = s.consService.Watch(w.logId, w); err != nil {
		return err
	}
	return w.Ready(ctx)
}

func (s *service) UnWatch(aclId string) (err error) {
	logId, err := cidToByte(aclId)
	if err != nil {
		return
	}
	return s.consService.UnWatch(logId)
}
