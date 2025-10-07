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

type InvitePayload struct {
	InviteType  aclrecordproto.AclInviteType
	Permissions list.AclPermissions
}

type GetRecordsResponse struct {
	Records []*consensusproto.RawRecordWithId
}

type AclSpaceClient interface {
	app.Component
	ReplaceInvite(ctx context.Context, invite InvitePayload) (list.InviteResult, error)
	ChangeInvitePermissions(ctx context.Context, inviteId string, permissions list.AclPermissions) error
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

func (c *aclSpaceClient) OwnershipChange(ctx context.Context, identity crypto.PubKey, oldOwnerPermissions list.AclPermissions) (err error) {
	c.acl.Lock()
	res, err := c.acl.RecordBuilder().BuildOwnershipChange(list.OwnershipChangePayload{
		NewOwner:            identity,
		OldOwnerPermissions: oldOwnerPermissions,
	})
	if err != nil {
		c.acl.Unlock()
		return err
	}
	c.acl.Unlock()
	return c.sendRecordAndUpdate(ctx, c.spaceId, res)
}

func (c *aclSpaceClient) ChangeInvitePermissions(ctx context.Context, inviteId string, permissions list.AclPermissions) error {
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

func (c *aclSpaceClient) ReplaceInvite(ctx context.Context, payload InvitePayload) (list.InviteResult, error) {
	c.acl.Lock()
	defer c.acl.Unlock()
	for _, invite := range c.acl.AclState().Invites() {
		if equalInvites(payload, invite) {
			return list.InviteResult{}, list.ErrDuplicateInvites
		}
	}
	var (
		inviteIds = c.acl.AclState().InviteIds()
		batch     list.BatchRequestPayload
	)
	if payload.InviteType == aclrecordproto.AclInviteType_RequestToJoin {
		batch = list.BatchRequestPayload{
			InviteRevokes: inviteIds,
			NewInvites:    []list.AclPermissions{list.AclPermissionsNone},
		}
	} else {
		batch = list.BatchRequestPayload{
			InviteRevokes: inviteIds,
			NewInvites:    []list.AclPermissions{payload.Permissions},
		}
	}
	res, err := c.acl.RecordBuilder().BuildBatchRequest(batch)
	if err != nil {
		return list.InviteResult{}, err
	}
	return list.InviteResult{
		InviteRec: res.Rec,
		InviteKey: res.Invites[0],
	}, nil
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

func equalInvites(payload InvitePayload, invite list.Invite) bool {
	switch payload.InviteType {
	case aclrecordproto.AclInviteType_RequestToJoin:
		return invite.Type == aclrecordproto.AclInviteType_RequestToJoin
	default:
		return invite.Type == aclrecordproto.AclInviteType_AnyoneCanJoin && invite.Permissions == payload.Permissions
	}
}
