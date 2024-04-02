//go:generate mockgen -destination mock_acl/mock_acl.go github.com/anyproto/any-sync/acl AclService
package acl

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	commonaccount "github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/app/ocache"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/consensus/consensusclient"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/metric"
	"github.com/anyproto/any-sync/util/crypto"
)

const CName = "coordinator.acl"

var log = logger.NewNamed(CName)

var ErrLimitExceed = errors.New("limit exceed")

type Limits struct {
	ReadMembers  uint32
	WriteMembers uint32
}

func New() AclService {
	return &aclService{}
}

type AclService interface {
	AddRecord(ctx context.Context, spaceId string, rec *consensusproto.RawRecord, limits Limits) (result *consensusproto.RawRecordWithId, err error)
	RecordsAfter(ctx context.Context, spaceId, aclHead string) (result []*consensusproto.RawRecordWithId, err error)
	Permissions(ctx context.Context, identity crypto.PubKey, spaceId string) (res list.AclPermissions, err error)
	OwnerPubKey(ctx context.Context, spaceId string) (ownerIdentity crypto.PubKey, err error)
	ReadState(ctx context.Context, spaceId string, f func(s *list.AclState) error) (err error)
	HasRecord(ctx context.Context, spaceId, recordId string) (has bool, err error)
	app.ComponentRunnable
}

type aclService struct {
	consService    consensusclient.Service
	cache          ocache.OCache
	accountService commonaccount.Service
}

func (as *aclService) Init(a *app.App) (err error) {
	as.consService = app.MustComponent[consensusclient.Service](a)
	as.accountService = app.MustComponent[commonaccount.Service](a)

	var metricReg *prometheus.Registry
	if m := a.Component(metric.CName); m != nil {
		metricReg = m.(metric.Metric).Registry()
	}
	as.cache = ocache.New(as.loadObject,
		ocache.WithTTL(5*time.Minute),
		ocache.WithLogger(log.Sugar()),
		ocache.WithPrometheus(metricReg, "acl", ""),
	)
	return
}

func (as *aclService) Name() (name string) {
	return CName
}

func (as *aclService) loadObject(ctx context.Context, id string) (ocache.Object, error) {
	return as.newAclObject(ctx, id)
}

func (as *aclService) get(ctx context.Context, spaceId string) (list.AclList, error) {
	obj, err := as.cache.Get(ctx, spaceId)
	if err != nil {
		return nil, err
	}
	aObj := obj.(*aclObject)
	aObj.lastUsage.Store(time.Now())
	return aObj.AclList, nil
}

func (as *aclService) AddRecord(ctx context.Context, spaceId string, rec *consensusproto.RawRecord, limits Limits) (result *consensusproto.RawRecordWithId, err error) {
	if limits.ReadMembers <= 1 && limits.WriteMembers <= 1 {
		return nil, ErrLimitExceed
	}

	acl, err := as.get(ctx, spaceId)
	if err != nil {
		return nil, err
	}
	acl.RLock()
	defer acl.RUnlock()

	var beforeReaders int
	for _, acc := range acl.AclState().CurrentAccounts() {
		if !acc.Permissions.NoPermissions() {
			beforeReaders++
		}
	}

	err = acl.ValidateRawRecord(rec, func(state *list.AclState) error {
		var readers, writers int
		for _, acc := range state.CurrentAccounts() {
			if acc.Permissions.NoPermissions() {
				continue
			}
			readers++
			if acc.Permissions.CanWrite() {
				writers++
			}
		}
		if readers >= beforeReaders {
			if uint32(readers) > limits.ReadMembers {
				return ErrLimitExceed
			}
			if uint32(writers) > limits.WriteMembers {
				return ErrLimitExceed
			}
		}
		return nil
	})
	if err != nil {
		return
	}

	return as.consService.AddRecord(ctx, spaceId, rec)
}

func (as *aclService) RecordsAfter(ctx context.Context, spaceId, aclHead string) (result []*consensusproto.RawRecordWithId, err error) {
	acl, err := as.get(ctx, spaceId)
	if err != nil {
		return nil, err
	}
	acl.RLock()
	defer acl.RUnlock()
	return acl.RecordsAfter(ctx, aclHead)
}

func (as *aclService) OwnerPubKey(ctx context.Context, spaceId string) (ownerIdentity crypto.PubKey, err error) {
	acl, err := as.get(ctx, spaceId)
	if err != nil {
		return
	}
	acl.RLock()
	defer acl.RUnlock()
	return acl.AclState().OwnerPubKey()
}

func (as *aclService) Permissions(ctx context.Context, identity crypto.PubKey, spaceId string) (res list.AclPermissions, err error) {
	acl, err := as.get(ctx, spaceId)
	if err != nil {
		return
	}
	acl.RLock()
	defer acl.RUnlock()
	return acl.AclState().Permissions(identity), nil
}

func (as *aclService) ReadState(ctx context.Context, spaceId string, f func(s *list.AclState) error) (err error) {
	acl, err := as.get(ctx, spaceId)
	if err != nil {
		return
	}
	acl.RLock()
	defer acl.RUnlock()
	return f(acl.AclState())
}

func (as *aclService) HasRecord(ctx context.Context, spaceId, recordId string) (has bool, err error) {
	acl, err := as.get(ctx, spaceId)
	if err != nil {
		return
	}
	acl.RLock()
	defer acl.RUnlock()
	acl.Iterate(func(record *list.AclRecord) (isContinue bool) {
		if record.Id == recordId {
			has = true
			return false
		}
		return true
	})
	return
}

func (as *aclService) Run(ctx context.Context) (err error) {
	return
}

func (as *aclService) Close(ctx context.Context) (err error) {
	return as.cache.Close()
}
