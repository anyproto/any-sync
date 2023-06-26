package list

import (
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/crypto"
)

type ContentValidator interface {
	ValidateAclRecordContents(ch *AclRecord) (err error)
	ValidatePermissionChange(ch *aclrecordproto.AclAccountPermissionChange, authorIdentity crypto.PubKey) (err error)
	ValidateInvite(ch *aclrecordproto.AclAccountInvite, authorIdentity crypto.PubKey) (err error)
	ValidateInviteRevoke(ch *aclrecordproto.AclAccountInviteRevoke, authorIdentity crypto.PubKey) (err error)
	ValidateRequestJoin(ch *aclrecordproto.AclAccountRequestJoin, authorIdentity crypto.PubKey) (err error)
	ValidateRequestAccept(ch *aclrecordproto.AclAccountRequestAccept, authorIdentity crypto.PubKey) (err error)
	ValidateRequestDecline(ch *aclrecordproto.AclAccountRequestDecline, authorIdentity crypto.PubKey) (err error)
	ValidateRemove(ch *aclrecordproto.AclAccountRemove, authorIdentity crypto.PubKey) (err error)
	ValidateReadKeyChange(ch *aclrecordproto.AclReadKeyChange, authorIdentity crypto.PubKey) (err error)
}

type contentValidator struct {
	keyStore crypto.KeyStorage
	aclState *AclState
}

func (c *contentValidator) ValidateAclRecordContents(ch *AclRecord) (err error) {
	if ch.PrevId != c.aclState.lastRecordId {
		return ErrIncorrectRecordSequence
	}
	aclData := ch.Model.(*aclrecordproto.AclData)
	for _, content := range aclData.AclContent {
		err = c.validateAclRecordContent(content, ch.Identity)
		if err != nil {
			return
		}
	}
	return
}

func (c *contentValidator) validateAclRecordContent(ch *aclrecordproto.AclContentValue, authorIdentity crypto.PubKey) (err error) {
	switch {
	case ch.GetPermissionChange() != nil:
		return c.ValidatePermissionChange(ch.GetPermissionChange(), authorIdentity)
	case ch.GetInvite() != nil:
		return c.ValidateInvite(ch.GetInvite(), authorIdentity)
	case ch.GetInviteRevoke() != nil:
		return c.ValidateInviteRevoke(ch.GetInviteRevoke(), authorIdentity)
	case ch.GetRequestJoin() != nil:
		return c.ValidateRequestJoin(ch.GetRequestJoin(), authorIdentity)
	case ch.GetRequestAccept() != nil:
		return c.ValidateRequestAccept(ch.GetRequestAccept(), authorIdentity)
	case ch.GetRequestDecline() != nil:
		return c.ValidateRequestDecline(ch.GetRequestDecline(), authorIdentity)
	case ch.GetAccountRemove() != nil:
		return c.ValidateRemove(ch.GetAccountRemove(), authorIdentity)
	case ch.GetReadKeyChange() != nil:
		return c.ValidateReadKeyChange(ch.GetReadKeyChange(), authorIdentity)
	default:
		return ErrUnexpectedContentType
	}
}

func (c *contentValidator) ValidatePermissionChange(ch *aclrecordproto.AclAccountPermissionChange, authorIdentity crypto.PubKey) (err error) {
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	chIdentity, err := c.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return err
	}
	_, exists := c.aclState.userStates[mapKeyFromPubKey(chIdentity)]
	if !exists {
		return ErrNoSuchAccount
	}
	return
}

func (c *contentValidator) ValidateInvite(ch *aclrecordproto.AclAccountInvite, authorIdentity crypto.PubKey) (err error) {
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	_, err = c.keyStore.PubKeyFromProto(ch.InviteKey)
	return
}

func (c *contentValidator) ValidateInviteRevoke(ch *aclrecordproto.AclAccountInviteRevoke, authorIdentity crypto.PubKey) (err error) {
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	_, exists := c.aclState.inviteKeys[ch.InviteRecordId]
	if !exists {
		return ErrNoSuchInvite
	}
	return
}

func (c *contentValidator) ValidateRequestJoin(ch *aclrecordproto.AclAccountRequestJoin, authorIdentity crypto.PubKey) (err error) {
	inviteKey, exists := c.aclState.inviteKeys[ch.InviteRecordId]
	if !exists {
		return ErrNoSuchInvite
	}
	inviteIdentity, err := c.keyStore.PubKeyFromProto(ch.InviteIdentity)
	if err != nil {
		return
	}
	if !authorIdentity.Equals(inviteIdentity) {
		return ErrIncorrectIdentity
	}
	rawInviteIdentity, err := inviteIdentity.Raw()
	if err != nil {
		return err
	}
	ok, err := inviteKey.Verify(rawInviteIdentity, ch.InviteIdentitySignature)
	if err != nil {
		return ErrInvalidSignature
	}
	if !ok {
		return ErrInvalidSignature
	}
	return
}

func (c *contentValidator) ValidateRequestAccept(ch *aclrecordproto.AclAccountRequestAccept, authorIdentity crypto.PubKey) (err error) {
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	record, exists := c.aclState.requestRecords[ch.RequestRecordId]
	if !exists {
		return ErrNoSuchRequest
	}
	acceptIdentity, err := c.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return
	}
	if !acceptIdentity.Equals(record.RequestIdentity) {
		return ErrIncorrectIdentity
	}
	if ch.Permissions == aclrecordproto.AclUserPermissions_Owner {
		return ErrInsufficientPermissions
	}
	return
}

func (c *contentValidator) ValidateRequestDecline(ch *aclrecordproto.AclAccountRequestDecline, authorIdentity crypto.PubKey) (err error) {
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	_, exists := c.aclState.requestRecords[ch.RequestRecordId]
	if !exists {
		return ErrNoSuchRequest
	}
	return
}

func (c *contentValidator) ValidateRemove(ch *aclrecordproto.AclAccountRemove, authorIdentity crypto.PubKey) (err error) {
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	identity, err := c.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return
	}
	_, exists := c.aclState.userStates[mapKeyFromPubKey(identity)]
	if !exists {
		return ErrNoSuchAccount
	}
	return c.validateAccountReadKeys(ch.AccountKeys)
}

func (c *contentValidator) ValidateReadKeyChange(ch *aclrecordproto.AclReadKeyChange, authorIdentity crypto.PubKey) (err error) {
	return c.validateAccountReadKeys(ch.AccountKeys)
}

func (c *contentValidator) validateAccountReadKeys(accountKeys []*aclrecordproto.AclEncryptedReadKey) (err error) {
	if len(accountKeys) != len(c.aclState.userStates) {
		return ErrIncorrectNumberOfAccounts
	}
	for _, encKeys := range accountKeys {
		identity, err := c.keyStore.PubKeyFromProto(encKeys.Identity)
		if err != nil {
			return err
		}
		_, exists := c.aclState.userStates[mapKeyFromPubKey(identity)]
		if !exists {
			return ErrNoSuchAccount
		}
	}
	return
}
