package commonspace

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/cache"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/diffservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/spacesyncproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/syncservice"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"hash/fnv"
	"math/rand"
	"time"
)

const CName = "common.commonspace"

var log = logger.NewNamed(CName)

func New() Service {
	return &service{}
}

type Service interface {
	CreateSpace(ctx context.Context, cache cache.TreeCache, payload SpaceCreatePayload) (Space, error)
	DeriveSpace(ctx context.Context, cache cache.TreeCache, payload SpaceDerivePayload) (Space, error)
	GetSpace(ctx context.Context, id string, cache cache.TreeCache) (sp Space, err error)
	app.Component
}

type service struct {
	config               config.Space
	configurationService nodeconf.Service
	storageProvider      storage.SpaceStorageProvider
}

func (s *service) Init(a *app.App) (err error) {
	s.config = a.MustComponent(config.CName).(*config.Config).Space
	s.storageProvider = a.MustComponent(storage.CName).(storage.SpaceStorageProvider)
	s.configurationService = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) CreateSpace(
	ctx context.Context,
	cache cache.TreeCache,
	payload SpaceCreatePayload) (sp Space, err error) {

	// unmarshalling signing and encryption keys
	identity, err := payload.SigningKey.GetPublic().Raw()
	if err != nil {
		return
	}
	encPubKey, err := payload.EncryptionKey.GetPublic().Raw()
	if err != nil {
		return
	}

	// preparing header and space id
	bytes := make([]byte, 32)
	_, err = rand.Read(bytes)
	if err != nil {
		return
	}
	header := &spacesyncproto.SpaceHeader{
		Identity:       identity,
		Timestamp:      time.Now().UnixNano(),
		SpaceType:      payload.SpaceType,
		ReplicationKey: payload.ReplicationKey,
		Seed:           bytes,
	}
	marshalled, err := header.Marshal()
	if err != nil {
		return
	}
	id, err := cid.NewCIDFromBytes(marshalled)
	if err != nil {
		return
	}
	spaceId := NewSpaceId(id, payload.ReplicationKey)

	// encrypting read key
	hasher := fnv.New64()
	_, err = hasher.Write(payload.ReadKey)
	if err != nil {
		return
	}
	readKeyHash := hasher.Sum64()
	encReadKey, err := payload.EncryptionKey.GetPublic().Encrypt(payload.ReadKey)
	if err != nil {
		return
	}

	// preparing acl
	aclRoot := &aclrecordproto.ACLRoot{
		Identity:           identity,
		EncryptionKey:      encPubKey,
		SpaceId:            spaceId,
		EncryptedReadKey:   encReadKey,
		DerivationScheme:   "",
		CurrentReadKeyHash: readKeyHash,
		Timestamp:          time.Now().UnixNano(),
	}
	rawWithId, err := marshalACLRoot(aclRoot, payload.SigningKey)
	if err != nil {
		return
	}

	// creating storage
	storageCreate := storage.SpaceStorageCreatePayload{
		RecWithId:   rawWithId,
		SpaceHeader: header,
		Id:          id,
	}
	_, err = s.storageProvider.CreateSpaceStorage(storageCreate)
	if err != nil {
		return
	}

	return s.GetSpace(ctx, spaceId, cache)
}

func (s *service) DeriveSpace(
	ctx context.Context,
	cache cache.TreeCache,
	payload SpaceDerivePayload) (sp Space, err error) {

	// unmarshalling signing and encryption keys
	identity, err := payload.SigningKey.GetPublic().Raw()
	if err != nil {
		return
	}
	signPrivKey, err := payload.SigningKey.Raw()
	if err != nil {
		return
	}
	encPubKey, err := payload.EncryptionKey.GetPublic().Raw()
	if err != nil {
		return
	}
	encPrivKey, err := payload.EncryptionKey.Raw()
	if err != nil {
		return
	}

	// preparing replication key
	hasher := fnv.New64()
	_, err = hasher.Write(identity)
	if err != nil {
		return
	}
	repKey := hasher.Sum64()

	// preparing header and space id
	header := &spacesyncproto.SpaceHeader{
		Identity:       identity,
		SpaceType:      SpaceTypeDerived,
		ReplicationKey: repKey,
	}
	marshalled, err := header.Marshal()
	if err != nil {
		return
	}
	id, err := cid.NewCIDFromBytes(marshalled)
	if err != nil {
		return
	}
	spaceId := NewSpaceId(id, repKey)

	// deriving and encrypting read key
	readKey, err := aclrecordproto.ACLReadKeyDerive(signPrivKey, encPrivKey)
	if err != nil {
		return
	}
	hasher = fnv.New64()
	_, err = hasher.Write(readKey.Bytes())
	if err != nil {
		return
	}
	readKeyHash := hasher.Sum64()
	encReadKey, err := payload.EncryptionKey.GetPublic().Encrypt(readKey.Bytes())
	if err != nil {
		return
	}

	// preparing acl
	aclRoot := &aclrecordproto.ACLRoot{
		Identity:           identity,
		EncryptionKey:      encPubKey,
		SpaceId:            spaceId,
		EncryptedReadKey:   encReadKey,
		DerivationScheme:   "",
		CurrentReadKeyHash: readKeyHash,
		Timestamp:          time.Now().UnixNano(),
	}
	rawWithId, err := marshalACLRoot(aclRoot, payload.SigningKey)
	if err != nil {
		return
	}

	// creating storage
	storageCreate := storage.SpaceStorageCreatePayload{
		RecWithId:   rawWithId,
		SpaceHeader: header,
		Id:          id,
	}
	_, err = s.storageProvider.CreateSpaceStorage(storageCreate)
	if err != nil {
		return
	}

	return s.GetSpace(ctx, spaceId, cache)
}

func (s *service) GetSpace(ctx context.Context, id string, cache cache.TreeCache) (Space, error) {
	st, err := s.storageProvider.SpaceStorage(id)
	if err != nil {
		return nil, err
	}
	lastConfiguration := s.configurationService.GetLast()
	diffService := diffservice.NewDiffService(id, s.config.SyncPeriod, st, lastConfiguration, cache, log)
	syncService := syncservice.NewSyncService(id, diffService, cache, lastConfiguration)
	sp := &space{
		id:          id,
		syncService: syncService,
		diffService: diffService,
		cache:       cache,
		storage:     st,
	}
	if err := sp.Init(ctx); err != nil {
		return nil, err
	}
	return sp, nil
}

func marshalACLRoot(aclRoot *aclrecordproto.ACLRoot, key signingkey.PrivKey) (rawWithId *aclrecordproto.RawACLRecordWithId, err error) {
	marshalledRoot, err := aclRoot.Marshal()
	if err != nil {
		return
	}
	signature, err := key.Sign(marshalledRoot)
	if err != nil {
		return
	}
	raw := &aclrecordproto.RawACLRecord{
		Payload:   marshalledRoot,
		Signature: signature,
	}
	marshalledRaw, err := raw.Marshal()
	if err != nil {
		return
	}
	aclHeadId, err := cid.NewCIDFromBytes(marshalledRaw)
	if err != nil {
		return
	}
	rawWithId = &aclrecordproto.RawACLRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	return
}
