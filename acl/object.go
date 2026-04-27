package acl

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func (as *aclService) newAclObject(ctx context.Context, id string) (*aclObject, error) {
	obj := &aclObject{
		id:         id,
		aclService: as,
		ready:      make(chan struct{}),
	}
	if err := as.consService.Watch(id, obj); err != nil {
		return nil, err
	}
	select {
	case <-obj.ready:
		if obj.consErr != nil {
			_ = as.consService.UnWatch(id)
			return nil, obj.consErr
		}
		return obj, nil
	case <-ctx.Done():
		_ = as.consService.UnWatch(id)
		return nil, ctx.Err()
	}
}

type aclObject struct {
	id         string
	aclService *aclService
	store      list.Storage

	list.AclList
	ready   chan struct{}
	consErr error

	lastUsage atomic.Time

	mu sync.Mutex
}

func (a *aclObject) AddConsensusRecords(recs []*consensusproto.RawRecordWithId) {
	a.mu.Lock()
	defer a.mu.Unlock()
	slices.Reverse(recs)
	if a.store == nil {
		defer close(a.ready)
		if a.store, a.consErr = list.NewInMemoryStorage(a.id, recs); a.consErr != nil {
			return
		}
		verifier := recordverifier.AcceptorVerifier(recordverifier.NewValidateFull())
		if networkId := a.aclService.nodeConf.Configuration().NetworkId; networkId != "" {
			netKey, err := crypto.DecodeNetworkId(networkId)
			if err != nil {
				a.consErr = fmt.Errorf("invalid networkId: %w", err)
				return
			}
			verifier = recordverifier.New(netKey)
		}
		if a.AclList, a.consErr = list.BuildAclListWithIdentity(a.aclService.accountService.Account(), a.store, verifier); a.consErr != nil {
			return
		}
	} else {
		a.Lock()
		defer a.Unlock()
		if err := a.AddRawRecords(recs); err != nil {
			log.Warn("unable to add consensus records", zap.Error(err), zap.String("spaceId", a.id))
			return
		}
	}
}

func (a *aclObject) AddConsensusError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.store == nil {
		a.consErr = err
		close(a.ready)
	} else {
		log.Warn("got consensus error", zap.Error(err))
	}
}

func (a *aclObject) Close() (err error) {
	return a.aclService.consService.UnWatch(a.id)
}

func (a *aclObject) TryClose(objectTTL time.Duration) (res bool, err error) {
	if a.lastUsage.Load().Before(time.Now().Add(-objectTTL)) {
		return true, a.Close()
	}
	return false, nil
}
