package syncacl

import (
	"context"
	"errors"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/headupdater"
	"github.com/anyproto/any-sync/commonspace/object/syncobjectgetter"

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

type SyncAcl interface {
	app.ComponentRunnable
	list.AclList
	syncobjectgetter.SyncObject
	SetHeadUpdater(updater headupdater.HeadUpdater)
	SyncWithPeer(ctx context.Context, peerId string) (err error)
}

func New() SyncAcl {
	return &syncAcl{}
}

type syncAcl struct {
	list.AclList
	syncClient  SyncClient
	syncHandler synchandler.SyncHandler
	headUpdater headupdater.HeadUpdater
	isClosed    bool
}

func (s *syncAcl) Run(ctx context.Context) (err error) {
	s.Lock()
	defer s.Unlock()
	s.headUpdater.UpdateHeads(s.Id(), []string{s.Head().Id})
	return
}

func (s *syncAcl) HandleRequest(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (response *spacesyncproto.ObjectSyncMessage, err error) {
	return s.syncHandler.HandleRequest(ctx, senderId, request)
}

func (s *syncAcl) SetHeadUpdater(updater headupdater.HeadUpdater) {
	s.headUpdater = updater
}

func (s *syncAcl) HandleMessage(ctx context.Context, senderId string, request *spacesyncproto.ObjectSyncMessage) (err error) {
	return s.syncHandler.HandleMessage(ctx, senderId, request)
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
	requestManager := a.MustComponent(requestmanager.CName).(requestmanager.RequestManager)
	peerManager := a.MustComponent(peermanager.CName).(peermanager.PeerManager)
	syncStatus := a.MustComponent(syncstatus.CName).(syncstatus.StatusService)
	s.syncClient = NewSyncClient(spaceId, requestManager, peerManager)
	s.syncHandler = newSyncAclHandler(storage.Id(), s, s.syncClient, syncStatus)
	return err
}

func (s *syncAcl) AddRawRecord(rawRec *consensusproto.RawRecordWithId) (err error) {
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

func (s *syncAcl) AddRawRecords(rawRecords []*consensusproto.RawRecordWithId) (err error) {
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

func (s *syncAcl) SyncWithPeer(ctx context.Context, peerId string) (err error) {
	s.Lock()
	defer s.Unlock()
	headUpdate := s.syncClient.CreateHeadUpdate(s, nil)
	return s.syncClient.SendUpdate(peerId, headUpdate)
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
