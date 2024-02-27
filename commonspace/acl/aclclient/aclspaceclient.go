package aclclient

import (
	"context"
	"errors"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/coordinator/coordinatorclient"
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
	StopSharing(ctx context.Context, readKeyChange list.ReadKeyChangePayload) (err error)
	AddRecord(ctx context.Context, consRec *consensusproto.RawRecord) error
	RemoveAccounts(ctx context.Context, payload list.AccountRemovePayload) error
	AcceptRequest(ctx context.Context, payload list.RequestAcceptPayload) error
	DeclineRequest(ctx context.Context, identity crypto.PubKey) (err error)
	CancelRequest(ctx context.Context) (err error)
	ChangePermissions(ctx context.Context, permChange list.PermissionChangesPayload) (err error)
	RequestSelfRemove(ctx context.Context) (err error)
	RevokeInvite(ctx context.Context, inviteRecordId string) (err error)
	RevokeAllInvites(ctx context.Context) (err error)
	AddAccounts(ctx context.Context, add list.AccountsAddPayload) (err error)
}

func NewAclSpaceClient() AclSpaceClient {
	return &aclSpaceClient{}
}

type aclSpaceClient struct {
	coordinatorClient coordinatorclient.CoordinatorClient
	acl               list.AclList
	spaceId           string
}

func (c *aclSpaceClient) Init(a *app.App) (err error) {
	c.coordinatorClient = a.MustComponent(coordinatorclient.CName).(coordinatorclient.CoordinatorClient)
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
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) RequestSelfRemove(ctx context.Context) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildRequestRemove()
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) ChangePermissions(ctx context.Context, permChange list.PermissionChangesPayload) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildPermissionChanges(permChange)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) AddAccounts(ctx context.Context, add list.AccountsAddPayload) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildAccountsAdd(add)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) RemoveAccounts(ctx context.Context, payload list.AccountRemovePayload) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildAccountRemove(payload)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) RevokeAllInvites(ctx context.Context) (err error) {
	c.acl.Lock()
	payload := list.BatchRequestPayload{
		InviteRevokes: c.acl.AclState().InviteIds(),
	}
	res, err := c.acl.RecordBuilder().BuildBatchRequest(payload)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) StopSharing(ctx context.Context, readKeyChange list.ReadKeyChangePayload) (err error) {
	c.acl.Lock()
	var (
		identities []crypto.PubKey
		recIds     []string
	)
	for _, state := range c.acl.AclState().CurrentAccounts() {
		if state.Permissions.NoPermissions() || state.Permissions.IsOwner() {
			continue
		}
		identities = append(identities, state.PubKey)
	}
	recs, _ := c.acl.AclState().JoinRecords(false)
	for _, rec := range recs {
		recIds = append(recIds, rec.RecordId)
	}
	payload := list.BatchRequestPayload{
		Removals: list.AccountRemovePayload{
			Identities: identities,
			Change:     readKeyChange,
		},
		Declines:      recIds,
		InviteRevokes: c.acl.AclState().InviteIds(),
	}
	res, err := c.acl.RecordBuilder().BuildBatchRequest(payload)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
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
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) CancelRequest(ctx context.Context) (err error) {
	c.acl.Lock()
	pendingReq, err := c.acl.AclState().Record(c.acl.AclState().Identity())
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
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) AcceptRequest(ctx context.Context, payload list.RequestAcceptPayload) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildRequestAccept(payload)
	if err != nil {
		c.acl.Unlock()
		return
	}
	c.acl.Unlock()
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) GenerateInvite() (resp list.InviteResult, err error) {
	c.acl.RLock()
	defer c.acl.RUnlock()
	return c.acl.RecordBuilder().BuildInvite()
}

func (c *aclSpaceClient) AddRecord(ctx context.Context, consRec *consensusproto.RawRecord) (err error) {
	return c.sendRecordAndUpdate(ctx, c.spaceId, consRec)
}

func (c *aclSpaceClient) sendRecordAndUpdate(ctx context.Context, spaceId string, rec *consensusproto.RawRecord) (err error) {
	res, err := c.coordinatorClient.AclAddRecord(ctx, spaceId, rec)
	if err != nil {
		return
	}
	c.acl.Lock()
	defer c.acl.Unlock()
	err = c.acl.AddRawRecord(res)
	if errors.Is(err, list.ErrRecordAlreadyExists) {
		err = nil
	}
	return
}
