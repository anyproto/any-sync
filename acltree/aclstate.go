package acltree

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/textileio/go-threads/crypto/symmetric"
)

var ErrNoSuchUser = errors.New("no such user")
var ErrFailedToDecrypt = errors.New("failed to decrypt key")
var ErrUserRemoved = errors.New("user was removed from the document")
var ErrDocumentForbidden = errors.New("your user was forbidden access to the document")
var ErrUserAlreadyExists = errors.New("user already exists")

type ACLState struct {
	currentReadKeyHash   uint64
	userReadKeys         map[uint64]*symmetric.Key
	userStates           map[string]*pb.ACLChangeUserState
	userInvites          map[string]*pb.ACLChangeUserInvite
	signingPubKeyDecoder keys.SigningPubKeyDecoder
	encryptionKey        keys.EncryptionPrivKey
	identity             string
}

func newACLState(
	identity string,
	encryptionKey keys.EncryptionPrivKey,
	signingPubKeyDecoder keys.SigningPubKeyDecoder) *ACLState {
	return &ACLState{
		identity:             identity,
		encryptionKey:        encryptionKey,
		userReadKeys:         make(map[uint64]*symmetric.Key),
		userStates:           make(map[string]*pb.ACLChangeUserState),
		userInvites:          make(map[string]*pb.ACLChangeUserInvite),
		signingPubKeyDecoder: signingPubKeyDecoder,
	}
}

func newACLStateFromSnapshotChange(
	snapshotChange *pb.ACLChange,
	identity string,
	encryptionKey keys.EncryptionPrivKey,
	signingPubKeyDecoder keys.SigningPubKeyDecoder) (*ACLState, error) {
	st := &ACLState{
		identity:             identity,
		encryptionKey:        encryptionKey,
		userReadKeys:         make(map[uint64]*symmetric.Key),
		userStates:           make(map[string]*pb.ACLChangeUserState),
		userInvites:          make(map[string]*pb.ACLChangeUserInvite),
		signingPubKeyDecoder: signingPubKeyDecoder,
	}
	err := st.recreateFromSnapshotChange(snapshotChange)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (st *ACLState) recreateFromSnapshotChange(snapshotChange *pb.ACLChange) error {
	snapshot := snapshotChange.GetAclData().GetAclSnapshot()
	if snapshot == nil {
		return fmt.Errorf("could not create state from snapshot, because it is nil")
	}
	state := snapshot.AclState
	for _, userState := range state.UserStates {
		st.userStates[userState.Identity] = userState
	}

	userState, exists := st.userStates[st.identity]
	if !exists {
		return ErrNoSuchUser
	}
	for _, key := range userState.EncryptedReadKeys {
		key, hash, err := st.decryptReadKeyAndHash(key)
		if err != nil {
			return ErrFailedToDecrypt
		}

		st.userReadKeys[hash] = key
	}
	st.currentReadKeyHash = snapshotChange.CurrentReadKeyHash
	if snapshot.GetAclState().GetInvites() != nil {
		st.userInvites = snapshot.GetAclState().GetInvites()
	}
	return nil
}

func (st *ACLState) makeSnapshot() *pb.ACLChangeACLSnapshot {
	var userStates []*pb.ACLChangeUserState
	for _, st := range st.userStates {
		userStates = append(userStates, st)
	}

	return &pb.ACLChangeACLSnapshot{AclState: &pb.ACLChangeACLState{
		ReadKeyHashes: nil,
		UserStates:    userStates, // TODO: make states and invites in same format
		Invites:       st.userInvites,
	}}
}

func (st *ACLState) applyChange(change *pb.ACLChange) (err error) {
	defer func() {
		if err != nil {
			return
		}
		st.currentReadKeyHash = change.CurrentReadKeyHash
	}()

	// we can't check this for the user which is joining, because it will not be in our list
	// the same is for the first change to be added
	skipIdentityCheck := st.isUserJoin(change) || (st.currentReadKeyHash == 0 && st.isUserAdd(change))
	if !skipIdentityCheck {
		// we check signature when we add this to the Tree, so no need to do it here
		if _, exists := st.userStates[change.Identity]; !exists {
			err = ErrNoSuchUser
			return
		}

		if !st.hasPermission(change.Identity, pb.ACLChange_Admin) {
			err = fmt.Errorf("user %s must have admin permissions", change.Identity)
			return
		}
	}

	for _, ch := range change.GetAclData().GetAclContent() {
		if err = st.applyChangeContent(ch); err != nil {
			//log.Infof("error while applying changes: %v; ignore", err)
			return err
		}
	}

	return nil
}

// TODO: remove changeId, because it is not needed
func (st *ACLState) applyChangeContent(ch *pb.ACLChangeACLContentValue) error {
	switch {
	case ch.GetUserPermissionChange() != nil:
		return st.applyUserPermissionChange(ch.GetUserPermissionChange())
	case ch.GetUserAdd() != nil:
		return st.applyUserAdd(ch.GetUserAdd())
	case ch.GetUserRemove() != nil:
		return st.applyUserRemove(ch.GetUserRemove())
	case ch.GetUserInvite() != nil:
		return st.applyUserInvite(ch.GetUserInvite())
	case ch.GetUserJoin() != nil:
		return st.applyUserJoin(ch.GetUserJoin())
	case ch.GetUserConfirm() != nil:
		return st.applyUserConfirm(ch.GetUserConfirm())
	default:
		return fmt.Errorf("unexpected change type: %v", ch)
	}
}

func (st *ACLState) applyUserPermissionChange(ch *pb.ACLChangeUserPermissionChange) error {
	if _, exists := st.userStates[ch.Identity]; !exists {
		return ErrNoSuchUser
	}

	st.userStates[ch.Identity].Permissions = ch.Permissions
	return nil
}

func (st *ACLState) applyUserInvite(ch *pb.ACLChangeUserInvite) error {
	st.userInvites[ch.InviteId] = ch
	return nil
}

func (st *ACLState) applyUserJoin(ch *pb.ACLChangeUserJoin) error {
	invite, exists := st.userInvites[ch.UserInviteId]
	if !exists {
		return fmt.Errorf("no such invite with id %s", ch.UserInviteId)
	}

	if _, exists = st.userStates[ch.Identity]; exists {
		return ErrUserAlreadyExists
	}

	// validating signature
	signature := ch.GetAcceptSignature()
	verificationKey, err := st.signingPubKeyDecoder.DecodeFromBytes(invite.AcceptPublicKey)
	if err != nil {
		return fmt.Errorf("public key verifying invite accepts is given in incorrect format: %v", err)
	}

	rawSignedId, err := st.signingPubKeyDecoder.DecodeFromStringIntoBytes(ch.Identity)
	if err != nil {
		return fmt.Errorf("failed to decode signing identity as bytes")
	}

	res, err := verificationKey.Verify(rawSignedId, signature)
	if err != nil {
		return fmt.Errorf("verification returned error: %w", err)
	}
	if !res {
		return fmt.Errorf("signature is invalid")
	}

	// if ourselves -> we need to decrypt the read keys
	if st.identity == ch.Identity {
		for _, key := range ch.EncryptedReadKeys {
			key, hash, err := st.decryptReadKeyAndHash(key)
			if err != nil {
				return ErrFailedToDecrypt
			}

			st.userReadKeys[hash] = key
		}
	}

	// adding user to the list
	userState := &pb.ACLChangeUserState{
		Identity:          ch.Identity,
		EncryptionKey:     ch.EncryptionKey,
		EncryptedReadKeys: ch.EncryptedReadKeys,
		Permissions:       invite.Permissions,
		IsConfirmed:       true,
	}
	st.userStates[ch.Identity] = userState
	return nil
}

func (st *ACLState) applyUserAdd(ch *pb.ACLChangeUserAdd) error {
	if _, exists := st.userStates[ch.Identity]; exists {
		return ErrUserAlreadyExists
	}

	st.userStates[ch.Identity] = &pb.ACLChangeUserState{
		Identity:          ch.Identity,
		EncryptionKey:     ch.EncryptionKey,
		Permissions:       ch.Permissions,
		EncryptedReadKeys: ch.EncryptedReadKeys,
	}

	if ch.Identity == st.identity {
		for _, key := range ch.EncryptedReadKeys {
			key, hash, err := st.decryptReadKeyAndHash(key)
			if err != nil {
				return ErrFailedToDecrypt
			}

			st.userReadKeys[hash] = key
		}
	}

	return nil
}

func (st *ACLState) applyUserRemove(ch *pb.ACLChangeUserRemove) error {
	if ch.Identity == st.identity {
		return ErrDocumentForbidden
	}

	if _, exists := st.userStates[ch.Identity]; !exists {
		return ErrNoSuchUser
	}

	delete(st.userStates, ch.Identity)

	for _, replace := range ch.ReadKeyReplaces {
		userState, exists := st.userStates[replace.Identity]
		if !exists {
			continue
		}

		userState.EncryptedReadKeys = append(userState.EncryptedReadKeys, replace.EncryptedReadKey)
		// if this is our identity then we have to decrypt the key
		if replace.Identity == st.identity {
			key, hash, err := st.decryptReadKeyAndHash(replace.EncryptedReadKey)
			if err != nil {
				return ErrFailedToDecrypt
			}

			st.currentReadKeyHash = hash
			st.userReadKeys[st.currentReadKeyHash] = key
		}
	}
	return nil
}

func (st *ACLState) applyUserConfirm(ch *pb.ACLChangeUserConfirm) error {
	if _, exists := st.userStates[ch.Identity]; !exists {
		return ErrNoSuchUser
	}

	userState := st.userStates[ch.Identity]
	userState.IsConfirmed = true
	return nil
}

func (st *ACLState) decryptReadKeyAndHash(msg []byte) (*symmetric.Key, uint64, error) {
	decrypted, err := st.encryptionKey.Decrypt(msg)
	if err != nil {
		return nil, 0, ErrFailedToDecrypt
	}

	key, err := symmetric.FromBytes(decrypted)
	if err != nil {
		return nil, 0, ErrFailedToDecrypt
	}

	hasher := fnv.New64()
	hasher.Write(decrypted)
	return key, hasher.Sum64(), nil
}

func (st *ACLState) hasPermission(identity string, permission pb.ACLChangeUserPermissions) bool {
	state, exists := st.userStates[identity]
	if !exists {
		return false
	}

	return state.Permissions == permission
}

func (st *ACLState) isUserJoin(ch *pb.ACLChange) bool {
	// if we have a UserJoin, then it should always be the first one applied
	return ch.AclData.GetAclContent() != nil && ch.AclData.GetAclContent()[0].GetUserJoin() != nil
}

func (st *ACLState) isUserAdd(ch *pb.ACLChange) bool {
	// if we have a UserAdd, then it should always be the first one applied
	userAdd := ch.AclData.GetAclContent()[0].GetUserAdd()
	return ch.AclData.GetAclContent() != nil && userAdd != nil && userAdd.GetIdentity() == ch.Identity
}

func (st *ACLState) getPermissionDecreasedUsers(ch *pb.ACLChange) (identities []*pb.ACLChangeUserPermissionChange) {
	// this should be called after general checks are completed
	if ch.GetAclData().GetAclContent() == nil {
		return nil
	}

	contents := ch.GetAclData().GetAclContent()
	for _, c := range contents {
		if c.GetUserPermissionChange() != nil {
			content := c.GetUserPermissionChange()

			currentState := st.userStates[content.Identity]
			// the comparison works in different direction :-)
			if content.Permissions > currentState.Permissions {
				identities = append(identities, &pb.ACLChangeUserPermissionChange{
					Identity:    content.Identity,
					Permissions: content.Permissions,
				})
			}
		}
		if c.GetUserRemove() != nil {
			content := c.GetUserRemove()
			identities = append(identities, &pb.ACLChangeUserPermissionChange{
				Identity:    content.Identity,
				Permissions: pb.ACLChange_Removed,
			})
		}
	}

	return identities
}

func (st *ACLState) equal(other *ACLState) bool {
	if st == nil && other == nil {
		return true
	}

	if st == nil || other == nil {
		return false
	}

	if st.currentReadKeyHash != other.currentReadKeyHash {
		return false
	}

	if st.identity != other.identity {
		return false
	}

	if len(st.userStates) != len(other.userStates) {
		return false
	}

	for _, st := range st.userStates {
		otherSt, exists := other.userStates[st.Identity]
		if !exists {
			return false
		}

		if st.Permissions != otherSt.Permissions {
			return false
		}

		if bytes.Compare(st.EncryptionKey, otherSt.EncryptionKey) != 0 {
			return false
		}
	}

	if len(st.userInvites) != len(other.userInvites) {
		return false
	}

	// TODO: add detailed user invites comparison + compare other stuff
	return true
}

func (st *ACLState) GetUserStates() map[string]*pb.ACLChangeUserState {
	// TODO: we should provide better API that would not allow to change this map from the outside
	return st.userStates
}
