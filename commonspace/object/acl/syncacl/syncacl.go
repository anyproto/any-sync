package syncacl

import (
	"context"
	"errors"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/objectsync/synchandler"
	"github.com/anyproto/any-sync/commonspace/peermanager"
	"github.com/anyproto/any-sync/commonspace/requestmanager"
	"github.com/anyproto/any-sync/commonspace/spacestorage"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/commonspace/syncstatus"
	"github.com/anyproto/any-sync/consensus/consensusproto"
)

const CName = "common.acl.syncacl"

var (
	log = logger.NewNamed(CName)

	ErrSyncAclClosed = errors.New("sync acl is closed")
)

func New() *SyncAcl {
	return &SyncAcl{}
}

type HeadUpdater interface {
	UpdateHeads(id string, heads []string)
}

type SyncAcl struct {
	list.AclList
	syncClient  SyncClient
	syncHandler synchandler.SyncHandler
	headUpdater HeadUpdater
	isClosed    bool
}

func (s *SyncAcl) Run(ctx context.Context) (err error) {
	return
}

func (s *SyncAcl) HandleRequest(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (response *spacesyncproto.ObjectSyncMessage, err error) {
	return s.HandleRequest(ctx, senderId, request)
}

func (s *SyncAcl) SetHeadUpdater(updater HeadUpdater) {
	s.headUpdater = updater
}

func (s *SyncAcl) HandleMessage(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (err error) {
	return s.HandleMessage(ctx, senderId, request)
}

func (s *SyncAcl) Init(a *app.App) (err error) {
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
	requestManager := a.MustComponent(requestmanager.CName).(requestmanager.RequestManager)
	peerManager := a.MustComponent(peermanager.CName).(peermanager.PeerManager)
	syncStatus := a.MustComponent(syncstatus.CName).(syncstatus.StatusService)
	s.syncClient = NewSyncClient(spaceId, requestManager, peerManager)
	s.syncHandler = newSyncAclHandler(storage.Id(), s, s.syncClient, syncStatus)
	return err
}

func (s *SyncAcl) AddRawRecord(rawRec *consensusproto.RawRecordWithId) (err error) {
	if s.isClosed {
		return ErrSyncAclClosed
	}
	err = s.AclList.AddRawRecord(rawRec)
	if err != nil {
		return
	}
	headUpdate := s.syncClient.CreateHeadUpdate(s, []*consensusproto.RawRecordWithId{rawRec})
	s.headUpdater.UpdateHeads(s.Id(), []string{rawRec.Id})
	s.syncClient.Broadcast(headUpdate)
	return
}

func (s *SyncAcl) AddRawRecords(rawRecords []*consensusproto.RawRecordWithId) (err error) {
	if s.isClosed {
		return ErrSyncAclClosed
	}
	err = s.AclList.AddRawRecords(rawRecords)
	if err != nil {
		return
	}
	headUpdate := s.syncClient.CreateHeadUpdate(s, rawRecords)
	s.headUpdater.UpdateHeads(s.Id(), []string{rawRecords[len(rawRecords)-1].Id})
	s.syncClient.Broadcast(headUpdate)
	return
}

func (s *SyncAcl) SyncWithPeer(ctx context.Context, peerId string) (err error) {
	s.Lock()
	defer s.Unlock()
	headUpdate := s.syncClient.CreateHeadUpdate(s, nil)
	return s.syncClient.SendUpdate(peerId, headUpdate)
}

func (s *SyncAcl) Close(ctx context.Context) (err error) {
	s.Lock()
	defer s.Unlock()
	s.isClosed = true
	return
}

func (s *SyncAcl) Name() (name string) {
	return CName
}
