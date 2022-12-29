package acl

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/accountservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/objectsync/synchandler"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/cidutil"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusclient"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/consensusproto"
	"time"
)

const CName = "node.acl"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type Service interface {
	CreateLog(ctx context.Context, aclId string, rawRec *aclrecordproto.RawAclRecord) (firstRecId string, err error)
	AddRecord(ctx context.Context, aclId string, rawRec *aclrecordproto.RawAclRecord) (id string, err error)
	Watch(ctx context.Context, spaceId, aclId string, h synchandler.SyncHandler) (err error)
	UnWatch(aclId string) (err error)
	app.Component
}

type service struct {
	consService consensusclient.Service
	account     accountservice.Service
}

func (s *service) Init(a *app.App) (err error) {
	s.consService = a.MustComponent(consensusclient.CName).(consensusclient.Service)
	s.account = a.MustComponent(accountservice.CName).(accountservice.Service)
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateLog(ctx context.Context, aclId string, rawRec *aclrecordproto.RawAclRecord) (firstRecId string, err error) {
	logId, err := cidToByte(aclId)
	if err != nil {
		return
	}
	recId, _, payload, err := s.signAndMarshal(rawRec)
	if err != nil {
		return
	}
	if err = s.consService.AddLog(ctx, &consensusproto.Log{
		Id: logId,
		Records: []*consensusproto.Record{
			{
				Id:          recId,
				Payload:     payload,
				CreatedUnix: uint64(time.Now().Unix()),
			},
		},
	}); err != nil {
		return
	}
	return cidToString(recId)
}

func (s *service) AddRecord(ctx context.Context, aclId string, rawRec *aclrecordproto.RawAclRecord) (id string, err error) {
	logId, err := cidToByte(aclId)
	if err != nil {
		return
	}

	recId, prevId, payload, err := s.signAndMarshal(rawRec)
	if err != nil {
		return
	}
	if err = s.consService.AddRecord(ctx, logId, &consensusproto.Record{
		Id:          recId,
		PrevId:      prevId,
		Payload:     payload,
		CreatedUnix: uint64(time.Now().Unix()),
	}); err != nil {
		return
	}
	return cidToString(recId)
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

func (s *service) signAndMarshal(rawRec *aclrecordproto.RawAclRecord) (recId, prevId, payload []byte, err error) {
	var rec = &aclrecordproto.AclRecord{}
	if err = rec.Unmarshal(rawRec.Payload); err != nil {
		return
	}
	if rec.PrevId != "" {
		if prevId, err = cidToByte(rec.PrevId); err != nil {
			return
		}
	}
	rawRec.AcceptorIdentity = s.account.Account().Identity
	if rawRec.AcceptorSignature, err = s.account.Account().SignKey.Sign(rawRec.Payload); err != nil {
		return
	}
	if payload, err = rawRec.Marshal(); err != nil {
		return
	}
	recCid, err := cidutil.NewCidFromBytes(payload)
	if err != nil {
		return
	}
	recId, err = cidToByte(recCid)
	return
}
