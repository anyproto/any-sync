package aclclient

import (
	"context"
	"fmt"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/liststorage"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/coordinator/coordinatorclient"
)

const CName = "common.acl.aclclient"

type AclJoiningClient interface {
	app.Component
	AclGetRecords(ctx context.Context, spaceId, aclHead string) ([]*consensusproto.RawRecordWithId, error)
	RequestJoin(ctx context.Context, spaceId string, payload list.RequestJoinPayload) error
	CancelRemoveSelf(ctx context.Context, spaceId string) (err error)
}

type aclJoiningClient struct {
	coordinatorClient coordinatorclient.CoordinatorClient
	keys              *accountdata.AccountKeys
}

func NewAclJoiningClient() AclJoiningClient {
	return &aclJoiningClient{}
}

func (c *aclJoiningClient) Name() (name string) {
	return CName
}

func (c *aclJoiningClient) Init(a *app.App) (err error) {
	c.coordinatorClient = a.MustComponent(coordinatorclient.CName).(coordinatorclient.CoordinatorClient)
	c.keys = a.MustComponent(accountservice.CName).(accountservice.Service).Account()
	return nil
}

func (c *aclJoiningClient) AclGetRecords(ctx context.Context, spaceId, aclHead string) (recs []*consensusproto.RawRecordWithId, err error) {
	return c.coordinatorClient.AclGetRecords(ctx, spaceId, aclHead)
}

func (c *aclJoiningClient) RequestJoin(ctx context.Context, spaceId string, payload list.RequestJoinPayload) (err error) {
	res, err := c.AclGetRecords(ctx, spaceId, "")
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
	acl, err := list.BuildAclListWithIdentity(c.keys, storage, list.NoOpAcceptorVerifier{})
	if err != nil {
		return err
	}
	joinRecs, err := acl.AclState().JoinRecords(false)
	if err != nil {
		return err
	}
	for _, rec := range joinRecs {
		if rec.RequestIdentity.Equals(c.keys.SignKey.GetPublic()) {
			// that means that we already requested to join
			return nil
		}
	}
	rec, err := acl.RecordBuilder().BuildRequestJoin(payload)
	if err != nil {
		return
	}
	_, err = c.coordinatorClient.AclAddRecord(ctx, spaceId, rec)
	return
}

func (c *aclJoiningClient) CancelRemoveSelf(ctx context.Context, spaceId string) (err error) {
	res, err := c.AclGetRecords(ctx, spaceId, "")
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
	acl, err := list.BuildAclListWithIdentity(c.keys, storage, list.NoOpAcceptorVerifier{})
	if err != nil {
		return err
	}
	pendingReq, err := acl.AclState().Record(acl.AclState().Identity())
	if err != nil {
		if !acl.AclState().Permissions(acl.AclState().Identity()).NoPermissions() {
			return nil
		}
		return err
	}
	if pendingReq.Type != list.RequestTypeRemove {
		return list.ErrInsufficientPermissions
	}
	newRec, err := acl.RecordBuilder().BuildRequestCancel(pendingReq.RecordId)
	if err != nil {
		return
	}
	_, err = c.coordinatorClient.AclAddRecord(ctx, spaceId, newRec)
	return
}
