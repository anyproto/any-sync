package syncacl

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/headupdater"
	"github.com/anyproto/any-sync/commonspace/sync"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

const CName = "common.acl.syncacl"

var (
	log = logger.NewNamed(CName)

	ErrSyncAclClosed = errors.New("sync acl is closed")
)

type SyncAcl interface {
	app.ComponentRunnable
	list.AclList
	SetHeadUpdater(updater headupdater.HeadUpdater)
	SyncWithPeer(ctx context.Context, peerId string) (err error)
	SetAclUpdater(updater headupdater.AclUpdater)
}

func New() SyncAcl {
	return &syncAcl{}
}

type syncAcl struct {
	list.AclList
	syncdeps.ObjectSyncHandler
	syncClient  SyncClient
	headUpdater headupdater.HeadUpdater
	isClosed    bool
	aclUpdater  headupdater.AclUpdater
}

func (s *syncAcl) SetAclUpdater(updater headupdater.AclUpdater) {
	s.Lock()
	defer s.Unlock()
	s.aclUpdater = updater
}

func (s *syncAcl) Run(ctx context.Context) (err error) {
	s.Lock()
	defer s.Unlock()
	s.headUpdater.UpdateHeads(s.Id(), []string{s.Head().Id})
	return
}

func (s *syncAcl) SetHeadUpdater(updater headupdater.HeadUpdater) {
	s.headUpdater = updater
}

func (s *syncAcl) Init(a *app.App) (err error) {
	storage := a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
	aclStorage, err := storage.AclStorage()
	if err != nil {
		return err
	}
	acc := a.MustComponent(accountservice.CName).(accountservice.Service)
	s.AclList, err = list.BuildAclListWithIdentity(acc.Account(), aclStorage, list.NoOpAcceptorVerifier{})
	if err != nil {
		return
	}
	spaceId := storage.Id()
	syncService := a.MustComponent(sync.CName).(sync.SyncService)
	s.syncClient = NewSyncClient(spaceId, syncService)
	s.ObjectSyncHandler = newSyncAclHandler(storage.Id(), s, s.syncClient)
	return err
}

func (s *syncAcl) AddRawRecord(rawRec *consensusproto.RawRecordWithId) (err error) {
	if s.isClosed {
		return ErrSyncAclClosed
	}
	log.Debug("received update", zap.String("aclId", s.AclList.Id()), zap.String("prevHead", s.AclList.Head().Id), zap.String("newHead", rawRec.Id))
	err = s.AclList.AddRawRecord(rawRec)
	if err != nil {
		return
	}
	headUpdate := s.syncClient.CreateHeadUpdate(s, []*consensusproto.RawRecordWithId{rawRec})
	s.broadcast(headUpdate)
	s.headUpdater.UpdateHeads(s.Id(), []string{rawRec.Id})
	if s.aclUpdater != nil {
		s.aclUpdater.UpdateAcl(s)
	}
	return
}

func (s *syncAcl) broadcast(headUpdate *objectsync.HeadUpdate) {
	err := s.syncClient.Broadcast(context.Background(), headUpdate)
	if err != nil {
		log.Error("broadcast acl message error", zap.Error(err))
	}
}

func (s *syncAcl) AddRawRecords(rawRecords []*consensusproto.RawRecordWithId) (err error) {
	if s.isClosed {
		return ErrSyncAclClosed
	}
	prevHead := s.AclList.Head().Id
	log := log.With(zap.String("aclId", s.AclList.Id()), zap.String("prevHead", prevHead))
	log.Debug("received updates", zap.String("newHead", rawRecords[len(rawRecords)-1].Id))
	err = s.AclList.AddRawRecords(rawRecords)
	if err != nil || s.AclList.Head().Id == prevHead {
		return
	}
	log.Debug("records updated", zap.String("head", s.AclList.Head().Id), zap.Int("len(total)", len(s.AclList.Records())))
	headUpdate := s.syncClient.CreateHeadUpdate(s, rawRecords)
	s.headUpdater.UpdateHeads(s.Id(), []string{rawRecords[len(rawRecords)-1].Id})
	s.broadcast(headUpdate)
	if s.aclUpdater != nil {
		s.aclUpdater.UpdateAcl(s)
	}
	return
}

func (s *syncAcl) SyncWithPeer(ctx context.Context, peerId string) (err error) {
	s.Lock()
	defer s.Unlock()
	req := s.syncClient.CreateFullSyncRequest(peerId, s)
	return s.syncClient.QueueRequest(ctx, req)
}

func (s *syncAcl) Close(ctx context.Context) (err error) {
	if s.AclList == nil {
		return
	}
	s.Lock()
	defer s.Unlock()
	s.isClosed = true
	return
}

func (s *syncAcl) Name() (name string) {
	return CName
}
