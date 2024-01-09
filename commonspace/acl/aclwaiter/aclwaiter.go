package aclwaiter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/acl/aclclient"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/liststorage"
	"github.com/anyproto/any-sync/util/periodicsync"
)

const CName = "common.acl.aclwaiter"

var log = logger.NewNamed(CName)

const (
	checkIntervalSecs = 5
	timeout           = 20 * time.Second
)

type AclWaiter interface {
	app.ComponentRunnable
}

type aclWaiter struct {
	client aclclient.AclInvitingClient
	keys   *accountdata.AccountKeys

	periodicCall periodicsync.PeriodicSync

	acl        list.AclList
	spaceId    string
	prevHeadId string

	onFinish func() error
	finished bool
}

func New(spaceId string, onFinish func() error) AclWaiter {
	return &aclWaiter{
		spaceId:  spaceId,
		onFinish: onFinish,
	}
}

func (a *aclWaiter) Init(app *app.App) (err error) {
	a.client = app.MustComponent(aclclient.CName).(aclclient.AclInvitingClient)
	a.keys = app.MustComponent(accountservice.CName).(accountservice.Service).Account()
	a.periodicCall = periodicsync.NewPeriodicSync(checkIntervalSecs, timeout, a.loop, log.With(zap.String("spaceId", a.spaceId)))
	return nil
}

func (a *aclWaiter) Name() (name string) {
	return CName
}

func (a *aclWaiter) loop(ctx context.Context) error {
	if a.acl == nil {
		res, err := a.client.AclGetRecords(ctx, "")
		if err != nil {
			return err
		}
		if len(res) == 0 {
			return fmt.Errorf("acl not found")
		}
		storage, err := liststorage.NewInMemoryAclListStorage(res[0].Id, res)
		if err != nil {
			return err
		}
		acl, err := list.BuildAclListWithIdentity(a.keys, storage, list.NoOpAcceptorVerifier{})
		if err != nil {
			return err
		}
		a.acl = acl
		a.prevHeadId = acl.Head().Id
	} else {
		prevId := a.prevHeadId
		res, err := a.client.AclGetRecords(ctx, prevId)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			return nil
		}
		for _, rec := range res {
			err := a.acl.AddRawRecord(rec)
			if err != nil && !errors.Is(err, list.ErrRecordAlreadyExists) {
				return err
			}
		}
	}
	// if the user was added
	if !a.acl.AclState().Permissions(a.keys.SignKey.GetPublic()).NoPermissions() {
		if !a.finished {
			err := a.onFinish()
			if err == nil {
				a.finished = true
			} else {
				return err
			}
		}
	}
	return nil
}

func (a *aclWaiter) Run(ctx context.Context) (err error) {
	a.periodicCall.Run()
	return nil
}

func (a *aclWaiter) Close(ctx context.Context) error {
	a.periodicCall.Close()
	return nil
}
