package list

import (
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
)

type ContentValidator interface {
	ValidateAclRecordContents(ch *AclRecord) (err error)
	ValidatePermissionChange(ch *aclrecordproto.AclAccountPermissionChange, authorIdentity crypto.PubKey) (err error)
	ValidatePermissionChanges(ch *aclrecordproto.AclAccountPermissionChanges, authorIdentity crypto.PubKey) (err error)
	ValidateOwnershipChange(ch *aclrecordproto.AclOwnershipChange, authorIdentity crypto.PubKey) (err error)
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
	ValidateSpaceOptionsChange(ch *aclrecordproto.AclSpaceOptionsChange, authorIdentity crypto.PubKey) (err error)
	ValidateChildRegister(ch *aclrecordproto.AclChildRegister, authorIdentity crypto.PubKey) (err error)
	ValidateChildRegisterRevoke(ch *aclrecordproto.AclChildRegisterRevoke, authorIdentity crypto.PubKey) (err error)
	ValidateLegalOwnerUpdate(ch *aclrecordproto.AclLegalOwnerUpdate, authorIdentity crypto.PubKey) (err error)
	ValidateAccountRemoveNoRotate(ch *aclrecordproto.AclAccountRemoveNoRotate, authorIdentity crypto.PubKey) (err error)
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

func (c *contentValidator) ValidateOwnershipChange(ch *aclrecordproto.AclOwnershipChange, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).IsOwner() {
		return ErrInsufficientPermissions
	}
	identity, err := c.keyStore.PubKeyFromProto(ch.NewOwnerIdentity)
	if err != nil {
		return err
	}
	var (
		oldOwnerPerms = AclPermissions(ch.OldOwnerPermissions)
		newOwnerPerms = c.aclState.Permissions(identity)
	)
	if newOwnerPerms.NoPermissions() {
		return ErrNoSuchAccount
	}
	newOwnerStatus := c.aclState.accountStates[mapKeyFromPubKey(identity)]
	if newOwnerStatus.Status != StatusActive ||
		newOwnerPerms.IsOwner() ||
		oldOwnerPerms.IsOwner() ||
		oldOwnerPerms.NoPermissions() {
		return ErrInsufficientPermissions
	}
	return nil
}

func (c *contentValidator) ValidateAccountsAdd(ch *aclrecordproto.AclAccountsAdd, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	authorPerms := c.aclState.Permissions(authorIdentity)
	if !authorPerms.CanManageAccounts() {
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
		if perm.IsAdmin() && !authorPerms.IsOwner() {
			// only the owner can add an account at Admin level (FR1)
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
	case ch.GetSpaceOptionsChange() != nil:
		return c.ValidateSpaceOptionsChange(ch.GetSpaceOptionsChange(), authorIdentity)
	case ch.GetChildRegister() != nil:
		return c.ValidateChildRegister(ch.GetChildRegister(), authorIdentity)
	case ch.GetChildRegisterRevoke() != nil:
		return c.ValidateChildRegisterRevoke(ch.GetChildRegisterRevoke(), authorIdentity)
	case ch.GetLegalOwnerUpdate() != nil:
		return c.ValidateLegalOwnerUpdate(ch.GetLegalOwnerUpdate(), authorIdentity)
	case ch.GetAccountRemoveNoRotate() != nil:
		return c.ValidateAccountRemoveNoRotate(ch.GetAccountRemoveNoRotate(), authorIdentity)
	default:
		return ErrUnexpectedContentType
	}
}

func (c *contentValidator) ValidatePermissionChange(ch *aclrecordproto.AclAccountPermissionChange, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	authorPerms := c.aclState.Permissions(authorIdentity)
	if !authorPerms.CanManageAccounts() {
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

	if currentState.Permissions.IsAdmin() && !authorPerms.IsOwner() {
		// only the owner can revoke the Admin role from a user (FR2)
		return ErrInsufficientPermissions
	}

	if ch.Permissions == aclrecordproto.AclUserPermissions_Owner {
		// not supported
		// if we are going to support owner transfer, it should be done with a separate acl change so we can't have more than 1 owner at a time
		return ErrInsufficientPermissions
	}

	if ch.Permissions == aclrecordproto.AclUserPermissions_Admin && !authorPerms.IsOwner() {
		// only the owner can assign the Admin role (FR1)
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
	authorPerms := c.aclState.Permissions(authorIdentity)
	if !authorPerms.CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	permissions := AclPermissions(ch.Permissions)
	if ch.InviteType == aclrecordproto.AclInviteType_AnyoneCanJoin {
		if permissions.IsOwner() || permissions.NoPermissions() || permissions.IsGuest() {
			return ErrInsufficientPermissions
		}
		if permissions.IsAdmin() && !authorPerms.IsOwner() {
			// only the owner can issue an Admin-granting invite (FR1)
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
	authorPerms := c.aclState.Permissions(authorIdentity)
	if !authorPerms.CanManageAccounts() {
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
	if permissions.IsAdmin() && !authorPerms.IsOwner() {
		// only the owner can change invite permissions to Admin (FR1)
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
	authorPerms := c.aclState.Permissions(authorIdentity)
	if !authorPerms.CanManageAccounts() {
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
	if ch.Permissions == aclrecordproto.AclUserPermissions_Admin && !authorPerms.IsOwner() {
		// only the owner can approve a join request at Admin level (FR1)
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
	authorPerms := c.aclState.Permissions(authorIdentity)
	if !authorPerms.CanManageAccounts() {
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
		if permissions.IsAdmin() && !authorPerms.IsOwner() {
			// only the owner can remove an Admin (FR2 — revoking admin via removal)
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
	authorPerms := c.aclState.Permissions(authorIdentity)
	if !authorPerms.CanManageAccounts() {
		// a Writer may author the rotation that completes a pending keyless removal,
		// but only when the space opted in via AclSpaceOptions
		opts := c.aclState.CurrentOptions()
		editorAllowed := opts != nil && opts.EditorsCanCompleteKeylessRotation &&
			authorPerms.CanWrite() && c.aclState.HasPendingKeylessRemovals()
		if !editorAllowed {
			return ErrInsufficientPermissions
		}
	}
	return c.validateReadKeyChange(ch, nil)
}

func (c *contentValidator) ValidateSpaceOptionsChange(ch *aclrecordproto.AclSpaceOptionsChange, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).IsOwner() {
		return ErrInsufficientPermissions
	}
	return nil
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

func (c *contentValidator) ValidateChildRegister(ch *aclrecordproto.AclChildRegister, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	if opts := c.aclState.CurrentOptions(); opts != nil && opts.ChildrenCreationDisallowed {
		return ErrChildrenCreationDisallowed
	}
	if ch.ChildSpaceId == "" || ch.ChildAclRootId == "" {
		return ErrNoSuchChildRegistration
	}
	if AclPermissions(ch.OrgPermission).IsOwner() {
		return ErrIsOwner
	}
	if reg, ok := c.aclState.childRegistrations[ch.ChildSpaceId]; ok && !reg.Revoked {
		return ErrChildAlreadyRegistered
	}
	return nil
}

func (c *contentValidator) ValidateChildRegisterRevoke(ch *aclrecordproto.AclChildRegisterRevoke, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if !c.aclState.Permissions(authorIdentity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	reg, ok := c.aclState.childRegistrations[ch.ChildSpaceId]
	if !ok || reg.Revoked {
		return ErrNoSuchChildRegistration
	}
	return nil
}

// ValidateLegalOwnerUpdate checks the signature-induction chain: the first embedded parent
// ownership-change must be signed by the currently stored legal owner, each next one by the
// owner the previous one named, and the author of this record must be the final owner. Each
// proof must be bound to the parent acl (aclRootId == the child's pinned parentAclRootId), and
// already-consumed proof CIDs are rejected so a cycled ownership cannot be REPLAYED.
//
// This induction is self-contained (verifiable offline, e.g. by external-seat members who do
// not replicate the parent acl) but proofs are only author-signature-checked here — they are
// NOT verified to have been accepted into the parent acl. A stored legal owner could therefore
// mint a FRESH (never-accepted) ownership change to a key of its choosing. The authoritative
// guard against that is the coordinator gate (verifyKeylessGovernanceRecord), which requires an
// AclLegalOwnerUpdate to be authored by the parent's CURRENT owner; this client-side check is
// defense-in-depth. Fully closing the offline gap needs an acceptor-inclusion proof (not
// available on the ValidateFull path, whose VerifyAcceptor is a no-op).
func (c *contentValidator) ValidateLegalOwnerUpdate(ch *aclrecordproto.AclLegalOwnerUpdate, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if c.aclState.legalOwner == nil {
		return ErrNotChildSpace
	}
	if len(ch.OwnershipChanges) == 0 {
		return ErrInvalidLegalOwnerProof
	}
	var (
		expected = c.aclState.legalOwner
		inBatch  = map[string]struct{}{}
	)
	for _, raw := range ch.OwnershipChanges {
		author, newOwner, proofId, aclRootId, err := unmarshalOwnershipChangeProof(c.keyStore, raw)
		if err != nil {
			return err
		}
		// bind the proof to the parent space: an ownership change from any OTHER
		// acl (even one signed by the same key) must not advance this child's owner
		if aclRootId == "" || aclRootId != c.aclState.parentAclRootId {
			return ErrInvalidLegalOwnerProof
		}
		if !author.Equals(expected) {
			return ErrInvalidLegalOwnerProof
		}
		if _, consumed := c.aclState.consumedOwnershipProofs[proofId]; consumed {
			return ErrInvalidLegalOwnerProof
		}
		if _, dup := inBatch[proofId]; dup {
			return ErrInvalidLegalOwnerProof
		}
		inBatch[proofId] = struct{}{}
		expected = newOwner
	}
	if !authorIdentity.Equals(expected) {
		return ErrInsufficientPermissions
	}
	return nil
}

// unmarshalOwnershipChangeProof decodes one embedded parent acl record (consensusproto.RawRecord bytes),
// verifies the author signature over its payload and requires it to contain exactly one AclOwnershipChange.
// Returns the record author, the new owner it names and the record cid (the replay-guard key).
func unmarshalOwnershipChangeProof(keyStore crypto.KeyStorage, raw []byte) (author, newOwner crypto.PubKey, proofId, aclRootId string, err error) {
	rawRec := &consensusproto.RawRecord{}
	if err = rawRec.UnmarshalVT(raw); err != nil {
		return
	}
	rec := &consensusproto.Record{}
	if err = rec.UnmarshalVT(rawRec.Payload); err != nil {
		return
	}
	author, err = keyStore.PubKeyFromProto(rec.Identity)
	if err != nil {
		return
	}
	res, err := author.Verify(rawRec.Payload, rawRec.Signature)
	if err != nil {
		return
	}
	if !res {
		err = ErrInvalidSignature
		return
	}
	aclData := &aclrecordproto.AclData{}
	if err = aclData.UnmarshalVT(rec.Data); err != nil {
		return
	}
	if len(aclData.AclContent) != 1 || aclData.AclContent[0].GetOwnershipChange() == nil {
		err = ErrInvalidLegalOwnerProof
		return
	}
	ownershipChange := aclData.AclContent[0].GetOwnershipChange()
	newOwner, err = keyStore.PubKeyFromProto(ownershipChange.NewOwnerIdentity)
	if err != nil {
		return
	}
	aclRootId = ownershipChange.AclRootId
	proofId, err = cidutil.NewCidFromBytes(raw)
	return
}

// ValidateAccountRemoveNoRotate admits the keyless-governance removal: only the current legalOwner
// of a child (nested) space may author it, even though it holds no permissions in this acl.
func (c *contentValidator) ValidateAccountRemoveNoRotate(ch *aclrecordproto.AclAccountRemoveNoRotate, authorIdentity crypto.PubKey) (err error) {
	if !c.verifier.ShouldValidate() {
		return nil
	}
	if c.aclState.legalOwner == nil {
		return ErrNotChildSpace
	}
	if !authorIdentity.Equals(c.aclState.legalOwner) {
		return ErrInsufficientPermissions
	}
	if len(ch.Identities) == 0 {
		return ErrIncorrectNumberOfAccounts
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
			// the legalOwner governs members, not the child's owner; deleting the space is its lever there
			return ErrInsufficientPermissions
		}
		idKey := mapKeyFromPubKey(identity)
		if _, exists := seenIdentities[idKey]; exists {
			return ErrDuplicateAccounts
		}
		seenIdentities[idKey] = struct{}{}
	}
	return nil
}
