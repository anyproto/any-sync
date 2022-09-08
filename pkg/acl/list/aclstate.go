package list

import (
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/common"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/symmetric"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"hash/fnv"
)

var log = logger.NewNamed("acllist").Sugar()

var ErrNoSuchUser = errors.New("no such user")
var ErrFailedToDecrypt = errors.New("failed to decrypt key")
var ErrUserRemoved = errors.New("user was removed from the document")
var ErrDocumentForbidden = errors.New("your user was forbidden access to the document")
var ErrUserAlreadyExists = errors.New("user already exists")
var ErrNoSuchRecord = errors.New("no such record")
var ErrInsufficientPermissions = errors.New("insufficient permissions")
var ErrNoReadKey = errors.New("acl state doesn't have a read key")
var ErrInvalidSignature = errors.New("signature is invalid")

type UserPermissionPair struct {
	Identity   string
	Permission aclpb.ACLChangeUserPermissions
}

type ACLState struct {
	currentReadKeyHash uint64
	userReadKeys       map[uint64]*symmetric.Key
	userStates         map[string]*aclpb.ACLChangeUserState
	userInvites        map[string]*aclpb.ACLChangeUserInvite

	signingPubKeyDecoder keys.Decoder
	encryptionKey        encryptionkey.PrivKey

	identity            string
	permissionsAtRecord map[string][]UserPermissionPair

	keychain *common.Keychain
}

func newACLStateWithIdentity(
	identity string,
	encryptionKey encryptionkey.PrivKey,
	decoder keys.Decoder) *ACLState {
	return &ACLState{
		identity:             identity,
		encryptionKey:        encryptionKey,
		userReadKeys:         make(map[uint64]*symmetric.Key),
		userStates:           make(map[string]*aclpb.ACLChangeUserState),
		userInvites:          make(map[string]*aclpb.ACLChangeUserInvite),
		signingPubKeyDecoder: decoder,
		permissionsAtRecord:  make(map[string][]UserPermissionPair),
		keychain:             common.NewKeychain(),
	}
}

func newACLState(decoder keys.Decoder) *ACLState {
	return &ACLState{
		signingPubKeyDecoder: decoder,
		userReadKeys:         make(map[uint64]*symmetric.Key),
		userStates:           make(map[string]*aclpb.ACLChangeUserState),
		userInvites:          make(map[string]*aclpb.ACLChangeUserInvite),
		permissionsAtRecord:  make(map[string][]UserPermissionPair),
		keychain:             common.NewKeychain(),
	}
}

func (st *ACLState) CurrentReadKeyHash() uint64 {
	return st.currentReadKeyHash
}

func (st *ACLState) CurrentReadKey() (*symmetric.Key, error) {
	key, exists := st.userReadKeys[st.currentReadKeyHash]
	if !exists {
		return nil, ErrNoReadKey
	}
	return key, nil
}

func (st *ACLState) UserReadKeys() map[uint64]*symmetric.Key {
	return st.userReadKeys
}

func (st *ACLState) PermissionsAtRecord(id string, identity string) (UserPermissionPair, error) {
	permissions, ok := st.permissionsAtRecord[id]
	if !ok {
		log.Errorf("missing record at id %s", id)
		return UserPermissionPair{}, ErrNoSuchRecord
	}

	for _, perm := range permissions {
		if perm.Identity == identity {
			return perm, nil
		}
	}
	return UserPermissionPair{}, ErrNoSuchUser
}

func (st *ACLState) applyRecord(record *aclpb.Record) (err error) {
	aclData := &aclpb.ACLChangeACLData{}

	err = proto.Unmarshal(record.Data, aclData)
	if err != nil {
		return
	}

	err = st.applyChangeData(aclData, record.CurrentReadKeyHash, record.Identity)
	if err != nil {
		return
	}

	st.currentReadKeyHash = record.CurrentReadKeyHash
	return
}

func (st *ACLState) applyChangeAndUpdate(recordWrapper *Record) (err error) {
	var (
		change  = recordWrapper.Content
		aclData = &aclpb.ACLChangeACLData{}
	)

	if recordWrapper.Model != nil {
		aclData = recordWrapper.Model.(*aclpb.ACLChangeACLData)
	} else {
		err = proto.Unmarshal(change.Data, aclData)
		if err != nil {
			return
		}
		recordWrapper.Model = aclData
	}

	err = st.applyChangeData(aclData, recordWrapper.Content.CurrentReadKeyHash, recordWrapper.Content.Identity)
	if err != nil {
		return
	}

	// getting all permissions for users at record
	var permissions []UserPermissionPair
	for _, state := range st.userStates {
		permission := UserPermissionPair{
			Identity:   state.Identity,
			Permission: state.Permissions,
		}
		permissions = append(permissions, permission)
	}

	st.permissionsAtRecord[recordWrapper.Id] = permissions
	return nil
}

func (st *ACLState) applyChangeData(changeData *aclpb.ACLChangeACLData, hash uint64, identity string) (err error) {
	defer func() {
		if err != nil {
			return
		}
		st.currentReadKeyHash = hash
	}()

	// we can't check this for the user which is joining, because it will not be in our list
	// the same is for the first change to be added
	skipIdentityCheck := st.isUserJoin(changeData) || (st.currentReadKeyHash == 0 && st.isUserAdd(changeData, identity))
	if !skipIdentityCheck {
		// we check signature when we add this to the Tree, so no need to do it here
		if _, exists := st.userStates[identity]; !exists {
			err = ErrNoSuchUser
			return
		}

		if !st.hasPermission(identity, aclpb.ACLChange_Admin) {
			err = fmt.Errorf("user %s must have admin permissions", identity)
			return
		}
	}

	for _, ch := range changeData.GetAclContent() {
		if err = st.applyChangeContent(ch); err != nil {
			log.Info("error while applying changes: %v; ignore", zap.Error(err))
			return err
		}
	}

	return nil
}

func (st *ACLState) applyChangeContent(ch *aclpb.ACLChangeACLContentValue) error {
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

func (st *ACLState) applyUserPermissionChange(ch *aclpb.ACLChangeUserPermissionChange) error {
	if _, exists := st.userStates[ch.Identity]; !exists {
		return ErrNoSuchUser
	}

	st.userStates[ch.Identity].Permissions = ch.Permissions
	return nil
}

func (st *ACLState) applyUserInvite(ch *aclpb.ACLChangeUserInvite) error {
	st.userInvites[ch.InviteId] = ch
	return nil
}

func (st *ACLState) applyUserJoin(ch *aclpb.ACLChangeUserJoin) error {
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

	res, err := verificationKey.(signingkey.PubKey).Verify(rawSignedId, signature)
	if err != nil {
		return fmt.Errorf("verification returned error: %w", err)
	}
	if !res {
		return ErrInvalidSignature
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
	userState := &aclpb.ACLChangeUserState{
		Identity:          ch.Identity,
		EncryptionKey:     ch.EncryptionKey,
		EncryptedReadKeys: ch.EncryptedReadKeys,
		Permissions:       invite.Permissions,
		IsConfirmed:       true,
	}
	st.userStates[ch.Identity] = userState
	return nil
}

func (st *ACLState) applyUserAdd(ch *aclpb.ACLChangeUserAdd) error {
	if _, exists := st.userStates[ch.Identity]; exists {
		return ErrUserAlreadyExists
	}

	st.userStates[ch.Identity] = &aclpb.ACLChangeUserState{
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

func (st *ACLState) applyUserRemove(ch *aclpb.ACLChangeUserRemove) error {
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

func (st *ACLState) applyUserConfirm(ch *aclpb.ACLChangeUserConfirm) error {
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

func (st *ACLState) hasPermission(identity string, permission aclpb.ACLChangeUserPermissions) bool {
	state, exists := st.userStates[identity]
	if !exists {
		return false
	}

	return state.Permissions == permission
}

func (st *ACLState) isUserJoin(data *aclpb.ACLChangeACLData) bool {
	// if we have a UserJoin, then it should always be the first one applied
	return data.GetAclContent() != nil && data.GetAclContent()[0].GetUserJoin() != nil
}

func (st *ACLState) isUserAdd(data *aclpb.ACLChangeACLData, identity string) bool {
	// if we have a UserAdd, then it should always be the first one applied
	userAdd := data.GetAclContent()[0].GetUserAdd()
	return data.GetAclContent() != nil && userAdd != nil && userAdd.GetIdentity() == identity
}

func (st *ACLState) GetUserStates() map[string]*aclpb.ACLChangeUserState {
	return st.userStates
}
