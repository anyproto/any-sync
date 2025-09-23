//go:generate mockgen -destination mock_syncacl/mock_syncacl.go github.com/anyproto/any-sync/commonspace/object/acl/syncacl SyncClient,SyncAcl
package syncacl

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/headupdater"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/sync"
	"github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	"github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/peer"
)

const CName = "common.acl.syncacl"

var (
	log = logger.NewNamed(CName)

	ErrSyncAclClosed = errors.New("sync acl is closed")
)

type SyncAcl interface {
	app.ComponentRunnable
	list.AclList
	syncdeps.ObjectSyncHandler
	SyncWithPeer(ctx context.Context, p peer.Peer) (err error)
	SetAclUpdater(updater headupdater.AclUpdater)
}

func New() SyncAcl {
	return &syncAcl{}
}

type syncAcl struct {
	list.AclList
	syncdeps.ObjectSyncHandler
	syncClient SyncClient
	isClosed   bool
	aclUpdater headupdater.AclUpdater
}

func (s *syncAcl) SetAclUpdater(updater headupdater.AclUpdater) {
	s.Lock()
	defer s.Unlock()
	s.aclUpdater = updater
}

func (s *syncAcl) Run(ctx context.Context) (err error) {
	s.Lock()
	defer s.Unlock()
	return
}

func (s *syncAcl) Init(a *app.App) (err error) {
	storage := a.MustComponent(spacestorage.CName).(spacestorage.SpaceStorage)
	aclStorage, err := storage.AclStorage()
	if err != nil {
		return err
	}
	acc := a.MustComponent(accountservice.CName).(accountservice.Service)
	verifier := a.MustComponent(recordverifier.CName).(recordverifier.RecordVerifier)
	s.AclList, err = list.BuildAclListWithIdentity(acc.Account(), aclStorage, verifier)
	fmt.Printf("-- BuildAclListWithIdentity...\n")
	if err != nil {
		fmt.Printf("-- BuildAclListWithIdentity err:%s\n", err.Error())
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
	prevHead := s.AclList.Head().Id
	log := log.With(zap.String("aclId", s.AclList.Id()), zap.String("prevHead", prevHead))
	err = s.AclList.AddRawRecord(rawRec)
	if err != nil {
		return
	}
	log.Debug("acl record updated", zap.String("head", s.AclList.Head().Id), zap.Int("len(total)", len(s.AclList.Records())))
	headUpdate, err := s.syncClient.CreateHeadUpdate(s, []*consensusproto.RawRecordWithId{rawRec})
	if err != nil {
		return
	}
	s.broadcast(headUpdate)
	if s.aclUpdater != nil {
		s.aclUpdater.UpdateAcl(s)
	}
	return
}

func (s *syncAcl) broadcast(headUpdate *objectmessages.HeadUpdate) {
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
	err = s.AclList.AddRawRecords(rawRecords)
	if err != nil || s.AclList.Head().Id == prevHead {
		return
	}
	log.Debug("acl records updated", zap.String("head", s.AclList.Head().Id), zap.Int("len(total)", len(s.AclList.Records())))
	headUpdate, err := s.syncClient.CreateHeadUpdate(s, rawRecords)
	if err != nil {
		return
	}
	s.broadcast(headUpdate)
	if s.aclUpdater != nil {
		s.aclUpdater.UpdateAcl(s)
	}
	return
}

func (s *syncAcl) SyncWithPeer(ctx context.Context, p peer.Peer) (err error) {
	s.Lock()
	defer s.Unlock()
	req := s.syncClient.CreateFullSyncRequest(p.Id(), s)
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
