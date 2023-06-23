package list

import (
	"errors"
	"fmt"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var log = logger.NewNamedSugared("common.commonspace.acllist")

var (
	ErrNoSuchAccount             = errors.New("no such account")
	ErrIncorrectInviteKey        = errors.New("incorrect invite key")
	ErrIncorrectIdentity         = errors.New("incorrect identity")
	ErrFailedToDecrypt           = errors.New("failed to decrypt key")
	ErrUserRemoved               = errors.New("user was removed from the document")
	ErrDocumentForbidden         = errors.New("your user was forbidden access to the document")
	ErrUserAlreadyExists         = errors.New("user already exists")
	ErrNoSuchRecord              = errors.New("no such record")
	ErrNoSuchRequest             = errors.New("no such request")
	ErrNoSuchInvite              = errors.New("no such invite")
	ErrInsufficientPermissions   = errors.New("insufficient permissions")
	ErrIncorrectNumberOfAccounts = errors.New("incorrect number of accounts")
	ErrNoReadKey                 = errors.New("acl state doesn't have a read key")
	ErrInvalidSignature          = errors.New("signature is invalid")
	ErrIncorrectRoot             = errors.New("incorrect root")
	ErrIncorrectRecordSequence   = errors.New("incorrect prev id of a record")
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
	inviteKeys       map[string]crypto.PubKey
	requestRecords   map[string]RequestRecord
	key              crypto.PrivKey
	pubKey           crypto.PubKey
	keyStore         crypto.KeyStorage
	totalReadKeys    int

	lastRecordId string
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
		inviteKeys:     make(map[string]crypto.PubKey),
		requestRecords: make(map[string]RequestRecord),
	}, nil
}

func newAclState(id string) *AclState {
	return &AclState{
		id:             id,
		userReadKeys:   make(map[string]crypto.SymKey),
		userStates:     make(map[string]AclUserState),
		statesAtRecord: make(map[string][]AclUserState),
		inviteKeys:     make(map[string]crypto.PubKey),
		requestRecords: make(map[string]RequestRecord),
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
	return AclUserState{}, ErrNoSuchAccount
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
	// if the record is root record
	if record.Id == st.id {
		err = st.applyRoot(record)
		if err != nil {
			return
		}
		st.statesAtRecord[record.Id] = []AclUserState{
			st.userStates[mapKeyFromPubKey(record.Identity)],
		}
		return
	}
	// if the model is not cached
	if record.Model == nil {
		aclData := &aclrecordproto.AclData{}
		err = proto.Unmarshal(record.Data, aclData)
		if err != nil {
			return
		}
		record.Model = aclData
	}
	// applying records contents
	err = st.applyChangeData(record)
	if err != nil {
		return
	}
	// getting all states for users at record and saving them
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
		Permissions: AclPermissions(aclrecordproto.AclUserPermissions_Admin),
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
	if root.EncryptedReadKey == nil {
		readKey, err = st.deriveKey()
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
	model := record.Model.(*aclrecordproto.AclData)
	if !st.isUserJoin(model) && !st.Permissions(record.Identity).CanManageAccounts() {
		return ErrInsufficientPermissions
	}
	for _, ch := range model.GetAclContent() {
		if err = st.applyChangeContent(ch, record.Id); err != nil {
			log.Info("error while applying changes: %v; ignore", zap.Error(err))
			return err
		}
	}
	if record.ReadKeyId != st.currentReadKeyId {
		st.totalReadKeys++
		st.currentReadKeyId = record.ReadKeyId
	}
	return nil
}

func (st *AclState) applyChangeContent(ch *aclrecordproto.AclContentValue, recordId string) error {
	switch {
	case ch.GetPermissionChange() != nil:
		return st.applyPermissionChange(ch.GetPermissionChange(), recordId)
	case ch.GetInvite() != nil:
		return st.applyInvite(ch.GetInvite(), recordId)
	case ch.GetInviteRevoke() != nil:
		return st.applyUserRemove(ch.GetUserRemove(), recordId)
	case ch.GetUserInvite() != nil:
		return st.applyUserInvite(ch.GetUserInvite(), recordId)
	case ch.GetUserJoin() != nil:
		return st.applyUserJoin(ch.GetUserJoin(), recordId)
	default:
		return fmt.Errorf("unexpected change type: %v", ch)
	}
}

func (st *AclState) applyPermissionChange(ch *aclrecordproto.AclAccountPermissionChange, recordId string) error {
	chIdentity, err := st.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return err
	}
	state, exists := st.userStates[mapKeyFromPubKey(chIdentity)]
	if !exists {
		return ErrNoSuchAccount
	}
	state.Permissions = AclPermissions(ch.Permissions)
	return nil
}

func (st *AclState) applyInvite(ch *aclrecordproto.AclAccountInvite, recordId string) error {
	inviteKey, err := st.keyStore.PubKeyFromProto(ch.InviteKey)
	if err != nil {
		return err
	}
	st.inviteKeys[recordId] = inviteKey
	return nil
}

func (st *AclState) applyInviteRevoke(ch *aclrecordproto.AclAccountInviteRevoke, recordId string) error {

	delete(st.inviteKeys, ch.InviteRecordId)
	return nil
}

func (st *AclState) applyUserJoin(ch *aclrecordproto.AclUserJoin, recordId string) error {
	return nil
}

func (st *AclState) applyUserAdd(ch *aclrecordproto.AclUserAdd, recordId string) error {
	return nil
}

func (st *AclState) applyUserRemove(ch *aclrecordproto.AclUserRemove, recordId string) error {
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

func (st *AclState) Permissions(identity crypto.PubKey) AclPermissions {
	state, exists := st.userStates[mapKeyFromPubKey(identity)]
	if !exists {
		return AclPermissions(aclrecordproto.AclUserPermissions_None)
	}
	return state.Permissions
}

func (st *AclState) isUserJoin(data *aclrecordproto.AclData) bool {
	// if we have a UserJoin, then it should always be the first one applied
	return data.GetAclContent() != nil && data.GetAclContent()[0].GetUserJoin() != nil
}

func (st *AclState) UserStates() map[string]AclUserState {
	return st.userStates
}

func (st *AclState) Invite(acceptPubKey []byte) (invite *aclrecordproto.AclUserInvite, err error) {
	return
}

func (st *AclState) LastRecordId() string {
	return st.lastRecordId
}

func (st *AclState) deriveKey() (crypto.SymKey, error) {
	keyBytes, err := st.key.Raw()
	if err != nil {
		return nil, err
	}
	return crypto.DeriveSymmetricKey(keyBytes, crypto.AnysyncSpacePath)
}

func mapKeyFromPubKey(pubKey crypto.PubKey) string {
	return string(pubKey.Storage())
}
