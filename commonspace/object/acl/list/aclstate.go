package list

import (
	"errors"
	"fmt"

	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/any-sync/commonspace/object/keychain"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var log = logger.NewNamedSugared("common.commonspace.acllist")

var (
	ErrNoSuchUser              = errors.New("no such user")
	ErrFailedToDecrypt         = errors.New("failed to decrypt key")
	ErrUserRemoved             = errors.New("user was removed from the document")
	ErrDocumentForbidden       = errors.New("your user was forbidden access to the document")
	ErrUserAlreadyExists       = errors.New("user already exists")
	ErrNoSuchRecord            = errors.New("no such record")
	ErrNoSuchInvite            = errors.New("no such invite")
	ErrOldInvite               = errors.New("invite is too old")
	ErrInsufficientPermissions = errors.New("insufficient permissions")
	ErrNoReadKey               = errors.New("acl state doesn't have a read key")
	ErrInvalidSignature        = errors.New("signature is invalid")
	ErrIncorrectRoot           = errors.New("incorrect root")
	ErrIncorrectRecordSequence = errors.New("incorrect prev id of a record")
)

type UserPermissionPair struct {
	Identity   crypto.PubKey
	Permission aclrecordproto.AclUserPermissions
}

type AclState struct {
	id               string
	currentReadKeyId string
	userReadKeys     map[string]crypto.SymKey
	userStates       map[string]AclUserState
	statesAtRecord   map[string][]AclUserState
	//userInvites      map[string]*aclrecordproto.AclUserInvite
	key           crypto.PrivKey
	pubKey        crypto.PubKey
	keyStore      crypto.KeyStorage
	totalReadKeys int

	lastRecordId string

	keychain *keychain.Keychain
}

func newAclStateWithKeys(
	id string,
	key crypto.PrivKey) (*AclState, error) {
	return &AclState{
		id:             id,
		key:            key,
		pubKey:         key.GetPublic(),
		userReadKeys:   make(map[string]crypto.SymKey),
		userStates:     make(map[string]AclUserState),
		statesAtRecord: make(map[string][]AclUserState),
		//userInvites:         make(map[string]*aclrecordproto.AclUserInvite),
	}, nil
}

func newAclState(id string) *AclState {
	return &AclState{
		id:             id,
		userReadKeys:   make(map[string]crypto.SymKey),
		userStates:     make(map[string]AclUserState),
		statesAtRecord: make(map[string][]AclUserState),
		//userInvites:         make(map[string]*aclrecordproto.AclUserInvite),
	}
}

func (st *AclState) CurrentReadKeyId() string {
	return st.currentReadKeyId
}

func (st *AclState) CurrentReadKey() (crypto.SymKey, error) {
	key, exists := st.userReadKeys[st.currentReadKeyId]
	if !exists {
		return nil, ErrNoReadKey
	}
	return key, nil
}

func (st *AclState) UserReadKeys() map[string]crypto.SymKey {
	return st.userReadKeys
}

func (st *AclState) StateAtRecord(id string, pubKey crypto.PubKey) (AclUserState, error) {
	userState, ok := st.statesAtRecord[id]
	if !ok {
		log.Errorf("missing record at id %s", id)
		return AclUserState{}, ErrNoSuchRecord
	}

	for _, perm := range userState {
		if perm.PubKey.Equals(pubKey) {
			return perm, nil
		}
	}
	return AclUserState{}, ErrNoSuchUser
}

func (st *AclState) applyRecord(record *AclRecord) (err error) {
	defer func() {
		if err == nil {
			st.lastRecordId = record.Id
		}
	}()
	if st.lastRecordId != record.PrevId {
		err = ErrIncorrectRecordSequence
		return
	}
	if record.Id == st.id {
		err = st.applyRoot(record)
		if err != nil {
			return
		}
		st.statesAtRecord[record.Id] = []AclUserState{
			{PubKey: record.Identity, Permissions: aclrecordproto.AclUserPermissions_Admin},
		}
		return
	}

	if record.Model == nil {
		aclData := &aclrecordproto.AclData{}
		err = proto.Unmarshal(record.Data, aclData)
		if err != nil {
			return
		}
		record.Model = aclData
	}

	err = st.applyChangeData(record)
	if err != nil {
		return
	}

	// getting all states for users at record
	var states []AclUserState
	for _, state := range st.userStates {
		states = append(states, state)
	}

	st.statesAtRecord[record.Id] = states
	return
}

func (st *AclState) applyRoot(record *AclRecord) (err error) {
	if st.key != nil && st.pubKey.Equals(record.Identity) {
		err = st.saveReadKeyFromRoot(record)
		if err != nil {
			return
		}
	}

	// adding user to the list
	userState := AclUserState{
		PubKey:      record.Identity,
		Permissions: aclrecordproto.AclUserPermissions_Admin,
	}
	st.currentReadKeyId = record.ReadKeyId
	st.userStates[mapKeyFromPubKey(record.Identity)] = userState
	st.totalReadKeys++
	return
}

func (st *AclState) saveReadKeyFromRoot(record *AclRecord) (err error) {
	var readKey crypto.SymKey
	root, ok := record.Model.(*aclrecordproto.AclRoot)
	if !ok {
		return ErrIncorrectRoot
	}
	if len(root.GetDerivationScheme()) != 0 {
		var keyBytes []byte
		keyBytes, err = st.key.Raw()
		if err != nil {
			return
		}

		readKey, err = crypto.DeriveAccountSymmetric(keyBytes)
		if err != nil {
			return
		}
	} else {
		readKey, err = st.decryptReadKey(root.EncryptedReadKey)
		if err != nil {
			return
		}
	}

	st.userReadKeys[record.Id] = readKey
	return
}

func (st *AclState) applyChangeData(record *AclRecord) (err error) {
	defer func() {
		if err != nil {
			return
		}
		if record.ReadKeyId != st.currentReadKeyId {
			st.totalReadKeys++
			st.currentReadKeyId = record.ReadKeyId
		}
	}()
	model := record.Model.(*aclrecordproto.AclData)
	if !st.isUserJoin(model) {
		// we check signature when we add this to the List, so no need to do it here
		if _, exists := st.userStates[mapKeyFromPubKey(record.Identity)]; !exists {
			err = ErrNoSuchUser
			return
		}

		// only Admins can do non-user join changes
		if !st.HasPermission(record.Identity, aclrecordproto.AclUserPermissions_Admin) {
			// TODO: add string encoding
			err = fmt.Errorf("user %s must have admin permissions", record.Identity.String())
			return
		}
	}

	for _, ch := range model.GetAclContent() {
		if err = st.applyChangeContent(ch, record.Id); err != nil {
			log.Info("error while applying changes: %v; ignore", zap.Error(err))
			return err
		}
	}

	return nil
}

func (st *AclState) applyChangeContent(ch *aclrecordproto.AclContentValue, recordId string) error {
	switch {
	case ch.GetUserPermissionChange() != nil:
		return st.applyUserPermissionChange(ch.GetUserPermissionChange(), recordId)
	case ch.GetUserAdd() != nil:
		return st.applyUserAdd(ch.GetUserAdd(), recordId)
	case ch.GetUserRemove() != nil:
		return st.applyUserRemove(ch.GetUserRemove(), recordId)
	case ch.GetUserInvite() != nil:
		return st.applyUserInvite(ch.GetUserInvite(), recordId)
	case ch.GetUserJoin() != nil:
		return st.applyUserJoin(ch.GetUserJoin(), recordId)
	default:
		return fmt.Errorf("unexpected change type: %v", ch)
	}
}

func (st *AclState) applyUserPermissionChange(ch *aclrecordproto.AclUserPermissionChange, recordId string) error {
	chIdentity, err := st.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return err
	}
	state, exists := st.userStates[mapKeyFromPubKey(chIdentity)]
	if !exists {
		return ErrNoSuchUser
	}

	state.Permissions = ch.Permissions
	return nil
}

func (st *AclState) applyUserInvite(ch *aclrecordproto.AclUserInvite, recordId string) error {
	//acceptPubKey, err := st.keyStore.PubKeyFromProto(ch.AcceptPublicKey)
	//if err != nil {
	//	return nil
	//}
	//st.userInvites[string(ch.AcceptPublicKey)] = ch
	return nil
}

func (st *AclState) applyUserJoin(ch *aclrecordproto.AclUserJoin, recordId string) error {
	//invite, exists := st.userInvites[string(ch.AcceptPubKey)]
	//if !exists {
	//	// TODO: change key to use same encoding
	//	return fmt.Errorf("no such invite with such public key %s", keys.EncodeBytesToString(ch.AcceptPubKey))
	//}
	//chIdentity := string(ch.Identity)
	//if _, exists = st.userStates[chIdentity]; exists {
	//	return ErrUserAlreadyExists
	//}
	//
	//// validating signature
	//signature := ch.GetAcceptSignature()
	//verificationKey, err := crypto.UnmarshalEd25519PublicKeyProto(invite.AcceptPublicKey)
	//if err != nil {
	//	return fmt.Errorf("public key verifying invite accepts is given in incorrect format: %v", err)
	//}
	//
	//// TODO: intuitively we need to sign not only the identity but a more complicated payload
	//res, err := verificationKey.Verify(ch.Identity, signature)
	//if err != nil {
	//	return fmt.Errorf("verification returned error: %w", err)
	//}
	//if !res {
	//	return ErrInvalidSignature
	//}
	//
	//// if ourselves -> we need to decrypt the read keys
	//if st.identity == chIdentity {
	//	for _, key := range ch.EncryptedReadKeys {
	//		key, err := st.decryptReadKey(key)
	//		if err != nil {
	//			return ErrFailedToDecrypt
	//		}
	//
	//		st.userReadKeys[recordId] = key
	//	}
	//}
	//
	//// adding user to the list
	//userState := &aclrecordproto.AclUserState{
	//	Identity:    ch.Identity,
	//	Permissions: invite.Permissions,
	//}
	//st.userStates[chIdentity] = userState
	return nil
}

func (st *AclState) applyUserAdd(ch *aclrecordproto.AclUserAdd, recordId string) error {
	//chIdentity := string(ch.Identity)
	//if _, exists := st.userStates[chIdentity]; exists {
	//	return ErrUserAlreadyExists
	//}
	//
	//st.userStates[chIdentity] = &aclrecordproto.AclUserState{
	//	Identity:      ch.Identity,
	//	EncryptionKey: ch.EncryptionKey,
	//	Permissions:   ch.Permissions,
	//}
	//
	//if chIdentity == st.identity {
	//	for _, key := range ch.EncryptedReadKeys {
	//		key, hash, err := st.decryptReadKey(key)
	//		if err != nil {
	//			return ErrFailedToDecrypt
	//		}
	//
	//		st.userReadKeys[hash] = key
	//	}
	//}

	return nil
}

func (st *AclState) applyUserRemove(ch *aclrecordproto.AclUserRemove, recordId string) error {
	//chIdentity := string(ch.Identity)
	//if chIdentity == st.identity {
	//	return ErrDocumentForbidden
	//}
	//
	//if _, exists := st.userStates[chIdentity]; !exists {
	//	return ErrNoSuchUser
	//}
	//
	//delete(st.userStates, chIdentity)
	//
	//for _, replace := range ch.ReadKeyReplaces {
	//	repIdentity := string(replace.Identity)
	//	// if this is our identity then we have to decrypt the key
	//	if repIdentity == st.identity {
	//		key, hash, err := st.decryptReadKey(replace.EncryptedReadKey)
	//		if err != nil {
	//			return ErrFailedToDecrypt
	//		}
	//
	//		st.userReadKeys[hash] = key
	//		break
	//	}
	//}
	return nil
}

func (st *AclState) decryptReadKey(msg []byte) (crypto.SymKey, error) {
	decrypted, err := st.key.Decrypt(msg)
	if err != nil {
		return nil, ErrFailedToDecrypt
	}

	key, err := crypto.UnmarshallAESKey(decrypted)
	if err != nil {
		return nil, ErrFailedToDecrypt
	}
	return key, nil
}

func (st *AclState) HasPermission(identity crypto.PubKey, permission aclrecordproto.AclUserPermissions) bool {
	state, exists := st.userStates[mapKeyFromPubKey(identity)]
	if !exists {
		return false
	}

	return state.Permissions == permission
}

func (st *AclState) isUserJoin(data *aclrecordproto.AclData) bool {
	// if we have a UserJoin, then it should always be the first one applied
	return data.GetAclContent() != nil && data.GetAclContent()[0].GetUserJoin() != nil
}

//func (st *AclState) isUserAdd(data *aclrecordproto.AclData, identity []byte) bool {
//	// if we have a UserAdd, then it should always be the first one applied
//	userAdd := data.GetAclContent()[0].GetUserAdd()
//	return data.GetAclContent() != nil && userAdd != nil && bytes.Compare(userAdd.GetIdentity(), identity) == 0
//}

func (st *AclState) UserStates() map[string]AclUserState {
	return st.userStates
}

//func (st *AclState) Invite(acceptPubKey []byte) (invite *aclrecordproto.AclUserInvite, err error) {
//	invite, exists := st.userInvites[string(acceptPubKey)]
//	if !exists {
//		err = ErrNoSuchInvite
//		return
//	}
//	if len(invite.EncryptedReadKeys) != st.totalReadKeys {
//		err = ErrOldInvite
//	}
//	return
//}

func (st *AclState) LastRecordId() string {
	return st.lastRecordId
}

func mapKeyFromPubKey(pubKey crypto.PubKey) string {
	return string(pubKey.Storage())
}
