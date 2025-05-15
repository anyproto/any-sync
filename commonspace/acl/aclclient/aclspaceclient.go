package aclclient

import (
	"context"
	"errors"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl"
	"github.com/anyproto/any-sync/commonspace/spacestate"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/node/nodeclient"
	"github.com/anyproto/any-sync/util/crypto"
)

type InviteResponse struct {
	InviteRec *consensusproto.RawRecord
	InviteKey crypto.PrivKey
}

type InviteChange struct {
	Perms list.AclPermissions
}

type GetRecordsResponse struct {
	Records []*consensusproto.RawRecordWithId
}

type InviteSaveFunc func()

type AclSpaceClient interface {
	app.Component
	GenerateInvite(shouldRevokeAll, isRequestToJoin bool, permissions list.AclPermissions) (list.InviteResult, error)
	ChangeInvite(ctx context.Context, inviteId string, permissions list.AclPermissions) error
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
	nodeClient nodeclient.NodeClient
	acl        list.AclList
	spaceId    string
}

func (c *aclSpaceClient) Init(a *app.App) (err error) {
	c.nodeClient = a.MustComponent(nodeclient.CName).(nodeclient.NodeClient)
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
	return c.sendRecordAndUpdate(ctx, c.spaceId, res.Rec)
}

func (c *aclSpaceClient) StopSharing(ctx context.Context, readKeyChange list.ReadKeyChangePayload) (err error) {
	c.acl.Lock()
	if c.acl.AclState().IsEmpty() {
		c.acl.Unlock()
		return
	}
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
	return c.sendRecordAndUpdate(ctx, c.spaceId, res.Rec)
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

func (c *aclSpaceClient) ChangeInvite(ctx context.Context, inviteId string, permissions list.AclPermissions) error {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildInviteChange(list.InviteChangePayload{
		IniviteRecordId: inviteId,
		Permissions:     permissions,
	})
	if err != nil {
		c.acl.Unlock()
		return err
	}
	c.acl.Unlock()
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) GenerateInvite(isRevoke, isRequestToJoin bool, permissions list.AclPermissions) (list.InviteResult, error) {
	c.acl.Lock()
	defer c.acl.Unlock()
	var inviteIds []string
	if isRevoke {
		for _, invite := range c.acl.AclState().Invites() {
			if isRequestToJoin && invite.Type == aclrecordproto.AclInviteType_RequestToJoin {
				return list.InviteResult{}, list.ErrDuplicateInvites
			} else if invite.Permissions == permissions {
				return list.InviteResult{}, list.ErrDuplicateInvites
			}
		}
		inviteIds = c.acl.AclState().InviteIds()
	}
	var payload list.BatchRequestPayload
	if isRequestToJoin {
		payload = list.BatchRequestPayload{
			InviteRevokes: inviteIds,
			NewInvites:    []list.AclPermissions{list.AclPermissionsNone},
		}
	} else {
		payload = list.BatchRequestPayload{
			InviteRevokes: inviteIds,
			NewInvites:    []list.AclPermissions{permissions},
		}
	}
	res, err := c.acl.RecordBuilder().BuildBatchRequest(payload)
	if err != nil {
		return list.InviteResult{}, err
	}
	return list.InviteResult{
		InviteRec: res.Rec,
		InviteKey: res.Invites[0],
	}, nil
}

func (c *aclSpaceClient) GenerateAnyoneCanJoinInvite(permissions list.AclPermissions) (resp list.InviteResult, err error) {
	c.acl.Lock()
	defer c.acl.Unlock()
	anyoneInvites := c.acl.AclState().Invites(aclrecordproto.AclInviteType_AnyoneCanJoin)
	if len(anyoneInvites) > 0 {
		return list.InviteResult{}, list.ErrDuplicateInvites
	}
	return c.acl.RecordBuilder().BuildInviteAnyone(permissions)
}

func (c *aclSpaceClient) AddRecord(ctx context.Context, consRec *consensusproto.RawRecord) (err error) {
	return c.sendRecordAndUpdate(ctx, c.spaceId, consRec)
}

func (c *aclSpaceClient) sendRecordAndUpdate(ctx context.Context, spaceId string, rec *consensusproto.RawRecord) (err error) {
	res, err := c.nodeClient.AclAddRecord(ctx, spaceId, rec)
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
