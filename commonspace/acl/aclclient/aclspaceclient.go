package aclclient

import (
	"context"
	"errors"

	"storj.io/drpc"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/crypto"
)

type InviteResponse struct {
	InviteRec *consensusproto.RawRecord
	InviteKey crypto.PrivKey
}

type GetRecordsResponse struct {
	Records []*consensusproto.RawRecordWithId
}

type InviteSaveFunc func()

type AclSpaceClient interface {
	app.Component
	GenerateInvite() (list.InviteResult, error)
	AddRecord(ctx context.Context, consRec *consensusproto.RawRecord) error
	RemoveAccounts(ctx context.Context, payload list.AccountRemovePayload) error
	AcceptRequest(ctx context.Context, payload list.RequestAcceptPayload) error
	DeclineRequest(ctx context.Context, identity crypto.PubKey) (err error)
	CancelRequest(ctx context.Context, identity crypto.PubKey) (err error)
	ChangePermissions(ctx context.Context, permChange list.PermissionChangesPayload) (err error)
	RequestSelfRemove(ctx context.Context) (err error)
	RevokeInvite(ctx context.Context, inviteRecordId string) (err error)
	AddAccounts(ctx context.Context, add list.AccountsAddPayload) (err error)
}

func NewAclSpaceClient() AclSpaceClient {
	return &aclSpaceClient{}
}

type aclSpaceClient struct {
	nodeConf nodeconf.Service
	pool     pool.Pool
	acl      list.AclList
	spaceId  string
}

func (c *aclSpaceClient) Init(a *app.App) (err error) {
	c.pool = a.MustComponent(pool.CName).(pool.Pool)
	c.nodeConf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	c.acl = a.MustComponent(syncacl.CName).(list.AclList)
	c.spaceId = a.MustComponent(spacestate.CName).(*spacestate.SpaceState).SpaceId
	return nil
}

func (c *aclSpaceClient) Name() (name string) {
	return CName
}

func (c *aclSpaceClient) RevokeInvite(ctx context.Context, inviteRecordId string) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildInviteRevoke(inviteRecordId)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	_, err = c.sendRecordAndUpdate(ctx, c.spaceId, res)
	return err
}

func (c *aclSpaceClient) RequestSelfRemove(ctx context.Context) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildRequestRemove()
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	_, err = c.sendRecordAndUpdate(ctx, c.spaceId, res)
	return err
}

func (c *aclSpaceClient) ChangePermissions(ctx context.Context, permChange list.PermissionChangesPayload) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildPermissionChanges(permChange)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	_, err = c.sendRecordAndUpdate(ctx, c.spaceId, res)
	return err
}

func (c *aclSpaceClient) AddAccounts(ctx context.Context, add list.AccountsAddPayload) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildAccountsAdd(add)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	_, err = c.sendRecordAndUpdate(ctx, c.spaceId, res)
	return err
}

func (c *aclSpaceClient) RemoveAccounts(ctx context.Context, payload list.AccountRemovePayload) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildAccountRemove(payload)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	_, err = c.sendRecordAndUpdate(ctx, c.spaceId, res)
	return err
}

func (c *aclSpaceClient) DeclineRequest(ctx context.Context, identity crypto.PubKey) (err error) {
	c.acl.Lock()
	pendingReq, err := c.acl.AclState().JoinRecord(identity, false)
	if err != nil {
		c.acl.Unlock()
		return
	}
	res, err := c.acl.RecordBuilder().BuildRequestDecline(pendingReq.RecordId)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	_, err = c.sendRecordAndUpdate(ctx, c.spaceId, res)
	return err
}

func (c *aclSpaceClient) CancelRequest(ctx context.Context, identity crypto.PubKey) (err error) {
	c.acl.Lock()
	pendingReq, err := c.acl.AclState().Record(identity)
	if err != nil {
		c.acl.Unlock()
		return
	}
	res, err := c.acl.RecordBuilder().BuildRequestCancel(pendingReq.RecordId)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	_, err = c.sendRecordAndUpdate(ctx, c.spaceId, res)
	return err
}

func (c *aclSpaceClient) AcceptRequest(ctx context.Context, payload list.RequestAcceptPayload) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildRequestAccept(payload)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	_, err = c.sendRecordAndUpdate(ctx, c.spaceId, res)
	return err
}

func (c *aclSpaceClient) GenerateInvite() (resp list.InviteResult, err error) {
	c.acl.RLock()
	defer c.acl.RUnlock()
	return c.acl.RecordBuilder().BuildInvite()
}

func (c *aclSpaceClient) AddRecord(ctx context.Context, consRec *consensusproto.RawRecord) (err error) {
	_, err = c.sendRecordAndUpdate(ctx, c.spaceId, consRec)
	return
}

func (c *aclSpaceClient) sendRecordAndUpdate(ctx context.Context, spaceId string, rec *consensusproto.RawRecord) (res *spacesyncproto.AclAddRecordResponse, err error) {
	marshalled, err := rec.Marshal()
	if err != nil {
		return
	}
	err = c.doClient(ctx, spaceId, func(cl spacesyncproto.DRPCSpaceSyncClient) error {
		res, err = cl.AclAddRecord(ctx, &spacesyncproto.AclAddRecordRequest{
			SpaceId: spaceId,
			Payload: marshalled,
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return
	}
	c.acl.Lock()
	defer c.acl.Unlock()
	err = c.acl.AddRawRecord(&consensusproto.RawRecordWithId{
		Payload: res.Payload,
		Id:      res.RecordId,
	})
	if errors.Is(err, list.ErrRecordAlreadyExists) {
		err = nil
	}
	return
}

func (c *aclSpaceClient) doClient(ctx context.Context, spaceId string, f func(cl spacesyncproto.DRPCSpaceSyncClient) error) error {
	p, err := c.pool.GetOneOf(ctx, c.nodeConf.NodeIds(spaceId))
	if err != nil {
		return err
	}
	return p.DoDrpc(ctx, func(conn drpc.Conn) error {
		return f(spacesyncproto.NewDRPCSpaceSyncClient(conn))
	})
}
