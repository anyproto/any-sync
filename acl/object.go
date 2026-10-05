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
	// ready is closed by the first consensus event, which leaves consErr set when the object failed to load
	ready   chan struct{}
	loaded  bool
	consErr error

	lastUsage atomic.Time

	mu sync.Mutex
}

// AddConsensusRecords builds the list from the first event and adds the records of the later ones.
// A watch can deliver more events after an error, which only finishes the load once.
func (a *aclObject) AddConsensusRecords(recs []*consensusproto.RawRecordWithId) {
	a.mu.Lock()
	defer a.mu.Unlock()
	slices.Reverse(recs)
	if !a.loaded {
		a.finishLoad(a.build(recs))
		return
	}
	if a.consErr != nil {
		// the object failed to load and is being dropped
		return
	}
	a.Lock()
	defer a.Unlock()
	if err := a.AddRawRecords(recs); err != nil {
		log.Warn("unable to add consensus records", zap.Error(err), zap.String("spaceId", a.id))
	}
}

func (a *aclObject) build(recs []*consensusproto.RawRecordWithId) (err error) {
	if a.store, err = list.NewInMemoryStorage(a.id, recs); err != nil {
		return err
	}
	verifier := recordverifier.AcceptorVerifier(recordverifier.NewValidateFull())
	if networkId := a.aclService.nodeConf.Configuration().NetworkId; networkId != "" {
		netKey, err := crypto.DecodeNetworkId(networkId)
		if err != nil {
			return fmt.Errorf("invalid networkId: %w", err)
		}
		verifier = recordverifier.New(netKey)
	}
	a.AclList, err = list.BuildAclListWithIdentity(a.aclService.accountService.Account(), a.store, verifier)
	return err
}

// finishLoad ends the wait in newAclObject with err; it is called once, under mu
func (a *aclObject) finishLoad(err error) {
	a.loaded = true
	a.consErr = err
	close(a.ready)
}

func (a *aclObject) AddConsensusError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.loaded {
		a.finishLoad(err)
	} else {
		log.Warn("got consensus error", zap.Error(err), zap.String("spaceId", a.id))
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
