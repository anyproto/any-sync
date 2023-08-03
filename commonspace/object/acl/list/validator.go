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
	ValidateAccountRemove(ch *aclrecordproto.AclAccountRemove, authorIdentity crypto.PubKey) (err error)
	ValidateRequestRemove(ch *aclrecordproto.AclAccountRequestRemove, authorIdentity crypto.PubKey) (err error)
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
		return c.ValidateAccountRemove(ch.GetAccountRemove(), authorIdentity)
	case ch.GetAccountRequestRemove() != nil:
		return c.ValidateRequestRemove(ch.GetAccountRequestRemove(), authorIdentity)
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
	_, exists := c.aclState.accountStates[mapKeyFromPubKey(chIdentity)]
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
	if _, exists := c.aclState.pendingRequests[mapKeyFromPubKey(inviteIdentity)]; exists {
		return ErrPendingRequest
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
	if len(ch.Metadata) > MaxMetadataLen {
		return ErrMetadataTooLarge
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

func (c *contentValidator) ValidateAccountRemove(ch *aclrecordproto.AclAccountRemove, authorIdentity crypto.PubKey) (err error) {
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	seenIdentities := map[string]struct{}{}
	for _, rawIdentity := range ch.Identities {
		identity, err := c.keyStore.PubKeyFromProto(rawIdentity)
		if err != nil {
			return err
		}
		if identity.Equals(authorIdentity) {
			return ErrInsufficientPermissions
		}
		permissions := c.aclState.Permissions(identity)
		if permissions.NoPermissions() {
			return ErrNoSuchAccount
		}
		if permissions.IsOwner() {
			return ErrInsufficientPermissions
		}
		idKey := mapKeyFromPubKey(identity)
		if _, exists := seenIdentities[idKey]; exists {
			return ErrDuplicateAccounts
		}
		seenIdentities[mapKeyFromPubKey(identity)] = struct{}{}
	}
	return c.validateReadKeyChange(ch.ReadKeyChange, seenIdentities)
}

func (c *contentValidator) ValidateRequestRemove(ch *aclrecordproto.AclAccountRequestRemove, authorIdentity crypto.PubKey) (err error) {
	if c.aclState.Permissions(authorIdentity).NoPermissions() {
		return ErrInsufficientPermissions
	}
	if _, exists := c.aclState.pendingRequests[mapKeyFromPubKey(authorIdentity)]; exists {
		return ErrPendingRequest
	}
	return
}

func (c *contentValidator) ValidateReadKeyChange(ch *aclrecordproto.AclReadKeyChange, authorIdentity crypto.PubKey) (err error) {
	return c.validateReadKeyChange(ch, nil)
}

func (c *contentValidator) validateReadKeyChange(ch *aclrecordproto.AclReadKeyChange, removedUsers map[string]struct{}) (err error) {
	_, err = c.keyStore.PubKeyFromProto(ch.MetadataPubKey)
	if err != nil {
		return ErrNoMetadataKey
	}
	if ch.EncryptedMetadataPrivKey == nil || ch.EncryptedOldReadKey == nil {
		return ErrIncorrectReadKey
	}
	for _, encKeys := range ch.AccountKeys {
		identity, err := c.keyStore.PubKeyFromProto(encKeys.Identity)
		if err != nil {
			return err
		}
		idKey := mapKeyFromPubKey(identity)
		_, exists := c.aclState.accountStates[idKey]
		if !exists {
			return ErrNoSuchAccount
		}
		if removedUsers == nil {
			continue
		}
		_, exists = removedUsers[idKey]
		if exists {
			return ErrIncorrectNumberOfAccounts
		}
	}
	return
}
