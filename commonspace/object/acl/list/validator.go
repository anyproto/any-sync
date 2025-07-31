package list

import (
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/util/crypto"
)

type ContentValidator interface {
	ValidateAclRecordContents(ch *AclRecord) (err error)
	ValidatePermissionChange(ch *aclrecordproto.AclAccountPermissionChange, authorIdentity crypto.PubKey) (err error)
	ValidatePermissionChanges(ch *aclrecordproto.AclAccountPermissionChanges, authorIdentity crypto.PubKey) (err error)
	ValidateAccountsAdd(ch *aclrecordproto.AclAccountsAdd, authorIdentity crypto.PubKey) (err error)
	ValidateInvite(ch *aclrecordproto.AclAccountInvite, authorIdentity crypto.PubKey) (err error)
	ValidateInviteJoin(ch *aclrecordproto.AclAccountInviteJoin, authorIdentity crypto.PubKey) (err error)
	ValidateInviteChange(ch *aclrecordproto.AclAccountInviteChange, authorIdentity crypto.PubKey) (err error)
	ValidateInviteRevoke(ch *aclrecordproto.AclAccountInviteRevoke, authorIdentity crypto.PubKey) (err error)
	ValidateRequestJoin(ch *aclrecordproto.AclAccountRequestJoin, authorIdentity crypto.PubKey) (err error)
	ValidateRequestAccept(ch *aclrecordproto.AclAccountRequestAccept, authorIdentity crypto.PubKey) (err error)
	ValidateRequestDecline(ch *aclrecordproto.AclAccountRequestDecline, authorIdentity crypto.PubKey) (err error)
	ValidateRequestCancel(ch *aclrecordproto.AclAccountRequestCancel, authorIdentity crypto.PubKey) (err error)
	ValidateAccountRemove(ch *aclrecordproto.AclAccountRemove, authorIdentity crypto.PubKey) (err error)
	ValidateRequestRemove(ch *aclrecordproto.AclAccountRequestRemove, authorIdentity crypto.PubKey) (err error)
	ValidateReadKeyChange(ch *aclrecordproto.AclReadKeyChange, authorIdentity crypto.PubKey) (err error)
}

type contentValidator struct {
	keyStore crypto.KeyStorage
	aclState *AclState
	verifier recordverifier.AcceptorVerifier
}

func newContentValidator(keyStore crypto.KeyStorage, aclState *AclState, verifier recordverifier.AcceptorVerifier) ContentValidator {
	return &contentValidator{
		keyStore: keyStore,
		aclState: aclState,
		verifier: verifier,
	}
}

func (c *contentValidator) ValidatePermissionChanges(ch *aclrecordproto.AclAccountPermissionChanges, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	for _, ch := range ch.Changes {
		err := c.ValidatePermissionChange(ch, authorIdentity)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *contentValidator) ValidateAccountsAdd(ch *aclrecordproto.AclAccountsAdd, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	for _, ch := range ch.Additions {
		identity, err := c.keyStore.PubKeyFromProto(ch.Identity)
		if err != nil {
			return err
		}
		if !c.aclState.Permissions(identity).NoPermissions() {
			return ErrDuplicateAccounts
		}
		perm := AclPermissions(ch.Permissions)
		if perm.IsOwner() {
			return ErrIsOwner
		}
		if perm.NoPermissions() {
			return ErrInsufficientPermissions
		}
	}
	return nil
}

func (c *contentValidator) ValidateInviteJoin(ch *aclrecordproto.AclAccountInviteJoin, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).NoPermissions() {
		return ErrInsufficientPermissions
	}
	invite, exists := c.aclState.invites[ch.InviteRecordId]
	if !exists {
		return ErrNoSuchInvite
	}
	if invite.Type != aclrecordproto.AclInviteType_AnyoneCanJoin {
		return ErrNoSuchInvite
	}
	if !AclPermissions(ch.Permissions).IsLessOrEqual(invite.Permissions) {
		return ErrInsufficientPermissions
	}
	if !c.aclState.Permissions(authorIdentity).NoPermissions() {
		return ErrInsufficientPermissions
	}
	inviteIdentity, err := c.keyStore.PubKeyFromProto(ch.Identity)
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
	ok, err := invite.Key.Verify(rawInviteIdentity, ch.InviteIdentitySignature)
	if err != nil {
		return ErrInvalidSignature
	}
	if !ok {
		return ErrInvalidSignature
	}
	if len(ch.Metadata) > MaxMetadataLen {
		return ErrMetadataTooLarge
	}
	if ch.EncryptedReadKey == nil {
		return ErrIncorrectReadKey
	}
	return nil
}

func (c *contentValidator) ValidateAclRecordContents(ch *AclRecord) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
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
	case ch.GetInviteJoin() != nil:
		return c.ValidateInviteJoin(ch.GetInviteJoin(), authorIdentity)
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
	case ch.GetPermissionChanges() != nil:
		return c.ValidatePermissionChanges(ch.GetPermissionChanges(), authorIdentity)
	case ch.GetAccountsAdd() != nil:
		return c.ValidateAccountsAdd(ch.GetAccountsAdd(), authorIdentity)
	default:
		return ErrUnexpectedContentType
	}
}

func (c *contentValidator) ValidatePermissionChange(ch *aclrecordproto.AclAccountPermissionChange, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	chIdentity, err := c.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return err
	}
	currentState, exists := c.aclState.accountStates[mapKeyFromPubKey(chIdentity)]
	if !exists {
		return ErrNoSuchAccount
	}

	if currentState.Permissions == AclPermissionsGuest {
		// it shouldn't be possible to change permission of guest user
		// it should be only possible to remove it with AccountRemove acl change
		return ErrInsufficientPermissions
	}

	if currentState.Permissions == AclPermissionsOwner {
		// it shouldn't be possible to change permission of owner
		return ErrInsufficientPermissions
	}

	if ch.Permissions == aclrecordproto.AclUserPermissions_Owner {
		// not supported
		// if we are going to support owner transfer, it should be done with a separate acl change so we can't have more than 1 owner at a time
		return ErrInsufficientPermissions
	}

	if ch.Permissions == aclrecordproto.AclUserPermissions_Guest && currentState.Permissions != AclPermissionsReader {
		// generally, it should be only possible to create guest user with AccountsAdd acl change
		// but in order to migrate the current guest users we allow to change permissions to guest from reader
		return ErrInsufficientPermissions
	}

	return
}

func (c *contentValidator) ValidateInvite(ch *aclrecordproto.AclAccountInvite, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	permissions := AclPermissions(ch.Permissions)
	if ch.InviteType == aclrecordproto.AclInviteType_AnyoneCanJoin {
		if permissions.IsOwner() || permissions.NoPermissions() || permissions.IsGuest() {
			return ErrInsufficientPermissions
		}
		if ch.EncryptedReadKey == nil {
			return ErrIncorrectReadKey
		}
	}
	_, err = c.keyStore.PubKeyFromProto(ch.InviteKey)
	return
}

func (c *contentValidator) ValidateInviteChange(ch *aclrecordproto.AclAccountInviteChange, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	invite, exists := c.aclState.invites[ch.InviteRecordId]
	if !exists {
		return ErrNoSuchInvite
	}
	permissions := AclPermissions(ch.Permissions)
	if invite.Type != aclrecordproto.AclInviteType_AnyoneCanJoin {
		return ErrNoSuchInvite
	}
	if invite.Permissions == permissions {
		return ErrInsufficientPermissions
	}
	if permissions.IsOwner() || permissions.NoPermissions() || permissions.IsGuest() {
		return ErrInsufficientPermissions
	}
	return
}

func (c *contentValidator) ValidateInviteRevoke(ch *aclrecordproto.AclAccountInviteRevoke, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	_, exists := c.aclState.invites[ch.InviteRecordId]
	if !exists {
		return ErrNoSuchInvite
	}
	return
}

func (c *contentValidator) ValidateRequestJoin(ch *aclrecordproto.AclAccountRequestJoin, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	invite, exists := c.aclState.invites[ch.InviteRecordId]
	if !exists {
		return ErrNoSuchInvite
	}
	if !c.aclState.Permissions(authorIdentity).NoPermissions() {
		return ErrInsufficientPermissions
	}
	if invite.Type != aclrecordproto.AclInviteType_RequestToJoin {
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
	ok, err := invite.Key.Verify(rawInviteIdentity, ch.InviteIdentitySignature)
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
	if !c.verifier.ShouldValidate() {
		return nil
	}
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
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	rec, exists := c.aclState.requestRecords[ch.RequestRecordId]
	if !exists || rec.Type != RequestTypeJoin {
		return ErrNoSuchRequest
	}
	return
}

func (c *contentValidator) ValidateRequestCancel(ch *aclrecordproto.AclAccountRequestCancel, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	rec, exists := c.aclState.requestRecords[ch.RecordId]
	if !exists {
		return ErrNoSuchRequest
	}
	if !rec.RequestIdentity.Equals(authorIdentity) {
		return ErrInsufficientPermissions
	}
	return
}

func (c *contentValidator) ValidateAccountRemove(ch *aclrecordproto.AclAccountRemove, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
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
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if c.aclState.Permissions(authorIdentity).NoPermissions() {
		return ErrInsufficientPermissions
	}
	if c.aclState.Permissions(authorIdentity).IsOwner() {
		return ErrIsOwner
	}
	if _, exists := c.aclState.pendingRequests[mapKeyFromPubKey(authorIdentity)]; exists {
		return ErrPendingRequest
	}
	return
}

func (c *contentValidator) ValidateReadKeyChange(ch *aclrecordproto.AclReadKeyChange, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	return c.validateReadKeyChange(ch, nil)
}

func (c *contentValidator) validateReadKeyChange(ch *aclrecordproto.AclReadKeyChange, removedUsers map[string]struct{}) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	_, err = c.keyStore.PubKeyFromProto(ch.MetadataPubKey)
	if err != nil {
		return ErrNoMetadataKey
	}
	if ch.EncryptedMetadataPrivKey == nil || ch.EncryptedOldReadKey == nil {
		return ErrIncorrectReadKey
	}
	var (
		activeUsers    []string
		updatedInvites []string
		activeInvites  []string
		updatedUsers   []string
	)
	for _, accState := range c.aclState.accountStates {
		if accState.Permissions.NoPermissions() {
			continue
		}
		pubKey := mapKeyFromPubKey(accState.PubKey)
		if _, exists := removedUsers[pubKey]; exists {
			continue
		}
		activeUsers = append(activeUsers, pubKey)
	}
	for _, invite := range c.aclState.invites {
		if invite.Type == aclrecordproto.AclInviteType_AnyoneCanJoin {
			activeInvites = append(activeInvites, mapKeyFromPubKey(invite.Key))
		}
	}
	for _, encKeys := range ch.AccountKeys {
		identity, err := c.keyStore.PubKeyFromProto(encKeys.Identity)
		if err != nil {
			return err
		}
		idKey := mapKeyFromPubKey(identity)
		updatedUsers = append(updatedUsers, idKey)
	}
	for _, invKeys := range ch.InviteKeys {
		identity, err := c.keyStore.PubKeyFromProto(invKeys.Identity)
		if err != nil {
			return err
		}
		idKey := mapKeyFromPubKey(identity)
		updatedInvites = append(updatedInvites, idKey)
	}
	if len(activeUsers) != len(updatedUsers) {
		return ErrIncorrectNumberOfAccounts
	}
	if len(activeInvites) != len(updatedInvites) {
		return ErrIncorrectNumberOfAccounts
	}
	slices.Sort(updatedUsers)
	slices.Sort(updatedInvites)
	slices.Sort(activeUsers)
	slices.Sort(activeInvites)
	if !slices.Equal(activeUsers, updatedUsers) || !slices.Equal(activeInvites, updatedInvites) {
		return ErrIncorrectNumberOfAccounts
	}
	return
}
