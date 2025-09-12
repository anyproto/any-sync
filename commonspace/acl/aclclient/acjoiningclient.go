//go:generate mockgen -destination mock_aclclient/mock_aclclient.go github.com/anyproto/any-sync/commonspace/acl/aclclient AclJoiningClient,AclSpaceClient
package aclclient

import (
	"context"
	"fmt"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/node/nodeclient"
)

const CName = "common.acl.aclclient"

type AclJoiningClient interface {
	app.Component
	AclGetRecords(ctx context.Context, spaceId, aclHead string) ([]*consensusproto.RawRecordWithId, error)
	RequestJoin(ctx context.Context, spaceId string, payload list.RequestJoinPayload) (aclHeadId string, err error)
	CancelJoin(ctx context.Context, spaceId string) (err error)
	InviteJoin(ctx context.Context, spaceId string, payload list.InviteJoinPayload) (aclHeadId string, err error)
	CancelRemoveSelf(ctx context.Context, spaceId string) (err error)
	RequestSelfRemove(ctx context.Context, spaceId string, aclList list.AclList) (err error)
}

type aclJoiningClient struct {
	nodeClient nodeclient.NodeClient
	keys       *accountdata.AccountKeys
}

func NewAclJoiningClient() AclJoiningClient {
	return &aclJoiningClient{}
}

func (c *aclJoiningClient) Name() (name string) {
	return CName
}

func (c *aclJoiningClient) Init(a *app.App) (err error) {
	c.nodeClient = a.MustComponent(nodeclient.CName).(nodeclient.NodeClient)
	c.keys = a.MustComponent(accountservice.CName).(accountservice.Service).Account()
	return nil
}

func (c *aclJoiningClient) AclGetRecords(ctx context.Context, spaceId, aclHead string) (recs []*consensusproto.RawRecordWithId, err error) {
	return c.nodeClient.AclGetRecords(ctx, spaceId, aclHead)
}

func (c *aclJoiningClient) getAcl(ctx context.Context, spaceId string) (l list.AclList, err error) {
	res, err := c.AclGetRecords(ctx, spaceId, "")
	if err != nil {
		return
	}
	if len(res) == 0 {
		err = fmt.Errorf("acl not found")
		return
	}
	storage, err := list.NewInMemoryStorage(res[0].Id, res)
	if err != nil {
		return
	}
	return list.BuildAclListWithIdentity(c.keys, storage, recordverifier.New())
}

func (c *aclJoiningClient) CancelJoin(ctx context.Context, spaceId string) (err error) {
	acl, err := c.getAcl(ctx, spaceId)
	if err != nil {
		return
	}
	pendingReq, err := acl.AclState().JoinRecord(acl.AclState().Identity(), false)
	if err != nil {
		return
	}
	res, err := acl.RecordBuilder().BuildRequestCancel(pendingReq.RecordId)
	if err != nil {
		return
	}
	_, err = c.nodeClient.AclAddRecord(ctx, spaceId, res)
	return
}

func (c *aclJoiningClient) RequestJoin(ctx context.Context, spaceId string, payload list.RequestJoinPayload) (aclHeadId string, err error) {
	acl, err := c.getAcl(ctx, spaceId)
	if err != nil {
		return
	}
	joinRecs, err := acl.AclState().JoinRecords(false)
	if err != nil {
		return
	}
	for _, rec := range joinRecs {
		if rec.RequestIdentity.Equals(c.keys.SignKey.GetPublic()) {
			// that means that we already requested to join
			return
		}
	}
	rec, err := acl.RecordBuilder().BuildRequestJoin(payload)
	if err != nil {
		return
	}
	recWithId, err := c.nodeClient.AclAddRecord(ctx, spaceId, rec)
	if err != nil {
		return
	}
	aclHeadId = recWithId.Id
	return
}

func (c *aclJoiningClient) InviteJoin(ctx context.Context, spaceId string, payload list.InviteJoinPayload) (aclHeadId string, err error) {
	acl, err := c.getAcl(ctx, spaceId)
	if err != nil {
		return
	}
	rec, err := acl.RecordBuilder().BuildInviteJoin(payload)
	if err != nil {
		return
	}
	recWithId, err := c.nodeClient.AclAddRecord(ctx, spaceId, rec)
	if err != nil {
		return
	}
	aclHeadId = recWithId.Id
	return
}

func (c *aclJoiningClient) CancelRemoveSelf(ctx context.Context, spaceId string) (err error) {
	acl, err := c.getAcl(ctx, spaceId)
	if err != nil {
		return
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
	_, err = c.nodeClient.AclAddRecord(ctx, spaceId, newRec)
	return
}

func (c *aclJoiningClient) RequestSelfRemove(ctx context.Context, spaceId string, aclList list.AclList) (err error) {
	aclList.Lock()
	res, err := aclList.RecordBuilder().BuildRequestRemove()
	if err != nil {
		aclList.Unlock()
		return
	}
	aclList.Unlock()
	rec, err := c.nodeClient.AclAddRecord(ctx, spaceId, res)
	if err != nil {
		return
	}
	aclList.Lock()
	defer aclList.Unlock()
	return aclList.AddRawRecord(rec)
}
