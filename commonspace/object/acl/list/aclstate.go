package list

import (
	"errors"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var log = logger.NewNamedSugared("common.commonspace.acllist")

var (
	ErrNoSuchAccount             = errors.New("no such account")
	ErrPendingRequest            = errors.New("already exists pending request")
	ErrUnexpectedContentType     = errors.New("unexpected content type")
	ErrIncorrectIdentity         = errors.New("incorrect identity")
	ErrIncorrectInviteKey        = errors.New("incorrect invite key")
	ErrFailedToDecrypt           = errors.New("failed to decrypt key")
	ErrNoSuchRecord              = errors.New("no such record")
	ErrNoSuchRequest             = errors.New("no such request")
	ErrNoSuchInvite              = errors.New("no such invite")
	ErrInsufficientPermissions   = errors.New("insufficient permissions")
	ErrIsOwner                   = errors.New("can't be made by owner")
	ErrIncorrectNumberOfAccounts = errors.New("incorrect number of accounts")
	ErrDuplicateAccounts         = errors.New("duplicate accounts")
	ErrNoReadKey                 = errors.New("acl state doesn't have a read key")
	ErrIncorrectReadKey          = errors.New("incorrect read key")
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
	// userReadKeys is a map recordId -> read key which tells us about every read key
	userReadKeys map[string]crypto.SymKey
	// userStates is a map pubKey -> state which defines current user state
	userStates map[string]AclUserState
	// statesAtRecord is a map recordId -> state which define user state at particular record
	//  probably this can grow rather large at some point, so we can maybe optimise later to have:
	//  - map pubKey -> []recordIds (where recordIds is an array where such identity permissions were changed)
	statesAtRecord map[string][]AclUserState
	// inviteKeys is a map recordId -> invite
	inviteKeys map[string]crypto.PubKey
	// requestRecords is a map recordId -> RequestRecord
	requestRecords map[string]RequestRecord
	// pendingRequests is a map pubKey -> RequestType
	pendingRequests map[string]RequestType
	key             crypto.PrivKey
	pubKey          crypto.PubKey
	keyStore        crypto.KeyStorage
	totalReadKeys   int

	lastRecordId     string
	contentValidator ContentValidator
}

func newAclStateWithKeys(
	id string,
	key crypto.PrivKey) (*AclState, error) {
	st := &AclState{
		id:              id,
		key:             key,
		pubKey:          key.GetPublic(),
		userReadKeys:    make(map[string]crypto.SymKey),
		userStates:      make(map[string]AclUserState),
		statesAtRecord:  make(map[string][]AclUserState),
		inviteKeys:      make(map[string]crypto.PubKey),
		requestRecords:  make(map[string]RequestRecord),
		pendingRequests: make(map[string]RequestType),
		keyStore:        crypto.NewKeyStorage(),
	}
	st.contentValidator = &contentValidator{
		keyStore: st.keyStore,
		aclState: st,
	}
	return st, nil
}

func newAclState(id string) *AclState {
	st := &AclState{
		id:              id,
		userReadKeys:    make(map[string]crypto.SymKey),
		userStates:      make(map[string]AclUserState),
		statesAtRecord:  make(map[string][]AclUserState),
		inviteKeys:      make(map[string]crypto.PubKey),
		requestRecords:  make(map[string]RequestRecord),
		pendingRequests: make(map[string]RequestType),
		keyStore:        crypto.NewKeyStorage(),
	}
	st.contentValidator = &contentValidator{
		keyStore: st.keyStore,
		aclState: st,
	}
	return st
}

func (st *AclState) Validator() ContentValidator {
	return st.contentValidator
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
		Permissions: AclPermissions(aclrecordproto.AclUserPermissions_Owner),
	}
	st.currentReadKeyId = record.Id
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
	for _, ch := range model.GetAclContent() {
		if err = st.applyChangeContent(ch, record.Id, record.Identity); err != nil {
			log.Info("error while applying changes: %v; ignore", zap.Error(err))
			return err
		}
	}
	return nil
}

func (st *AclState) applyChangeContent(ch *aclrecordproto.AclContentValue, recordId string, authorIdentity crypto.PubKey) error {
	switch {
	case ch.GetPermissionChange() != nil:
		return st.applyPermissionChange(ch.GetPermissionChange(), recordId, authorIdentity)
	case ch.GetInvite() != nil:
		return st.applyInvite(ch.GetInvite(), recordId, authorIdentity)
	case ch.GetInviteRevoke() != nil:
		return st.applyInviteRevoke(ch.GetInviteRevoke(), recordId, authorIdentity)
	case ch.GetRequestJoin() != nil:
		return st.applyRequestJoin(ch.GetRequestJoin(), recordId, authorIdentity)
	case ch.GetRequestAccept() != nil:
		return st.applyRequestAccept(ch.GetRequestAccept(), recordId, authorIdentity)
	case ch.GetRequestDecline() != nil:
		return st.applyRequestDecline(ch.GetRequestDecline(), recordId, authorIdentity)
	case ch.GetAccountRemove() != nil:
		return st.applyAccountRemove(ch.GetAccountRemove(), recordId, authorIdentity)
	case ch.GetReadKeyChange() != nil:
		return st.applyReadKeyChange(ch.GetReadKeyChange(), recordId, authorIdentity)
	case ch.GetAccountRequestRemove() != nil:
		return st.applyRequestRemove(ch.GetAccountRequestRemove(), recordId, authorIdentity)
	default:
		return ErrUnexpectedContentType
	}
}

func (st *AclState) applyPermissionChange(ch *aclrecordproto.AclAccountPermissionChange, recordId string, authorIdentity crypto.PubKey) error {
	chIdentity, err := st.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return err
	}
	err = st.contentValidator.ValidatePermissionChange(ch, authorIdentity)
	if err != nil {
		return err
	}
	stringKey := mapKeyFromPubKey(chIdentity)
	state, _ := st.userStates[stringKey]
	state.Permissions = AclPermissions(ch.Permissions)
	st.userStates[stringKey] = state
	return nil
}

func (st *AclState) applyInvite(ch *aclrecordproto.AclAccountInvite, recordId string, authorIdentity crypto.PubKey) error {
	inviteKey, err := st.keyStore.PubKeyFromProto(ch.InviteKey)
	if err != nil {
		return err
	}
	err = st.contentValidator.ValidateInvite(ch, authorIdentity)
	if err != nil {
		return err
	}
	st.inviteKeys[recordId] = inviteKey
	return nil
}

func (st *AclState) applyInviteRevoke(ch *aclrecordproto.AclAccountInviteRevoke, recordId string, authorIdentity crypto.PubKey) error {
	err := st.contentValidator.ValidateInviteRevoke(ch, authorIdentity)
	if err != nil {
		return err
	}
	delete(st.inviteKeys, ch.InviteRecordId)
	return nil
}

func (st *AclState) applyRequestJoin(ch *aclrecordproto.AclAccountRequestJoin, recordId string, authorIdentity crypto.PubKey) error {
	err := st.contentValidator.ValidateRequestJoin(ch, authorIdentity)
	if err != nil {
		return err
	}
	st.pendingRequests[mapKeyFromPubKey(authorIdentity)] = RequestTypeJoin
	st.requestRecords[recordId] = RequestRecord{
		RequestIdentity: authorIdentity,
		RequestMetadata: ch.Metadata,
	}
	return nil
}

func (st *AclState) applyRequestAccept(ch *aclrecordproto.AclAccountRequestAccept, recordId string, authorIdentity crypto.PubKey) error {
	err := st.contentValidator.ValidateRequestAccept(ch, authorIdentity)
	if err != nil {
		return err
	}
	acceptIdentity, err := st.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return err
	}
	record, _ := st.requestRecords[ch.RequestRecordId]
	st.userStates[mapKeyFromPubKey(acceptIdentity)] = AclUserState{
		PubKey:          acceptIdentity,
		Permissions:     AclPermissions(ch.Permissions),
		RequestMetadata: record.RequestMetadata,
	}
	delete(st.pendingRequests, mapKeyFromPubKey(st.requestRecords[ch.RequestRecordId].RequestIdentity))
	if !st.pubKey.Equals(acceptIdentity) {
		return nil
	}
	for _, key := range ch.EncryptedReadKeys {
		decrypted, err := st.key.Decrypt(key.EncryptedReadKey)
		if err != nil {
			return err
		}
		sym, err := crypto.UnmarshallAESKey(decrypted)
		if err != nil {
			return err
		}
		st.userReadKeys[key.RecordId] = sym
	}
	return nil
}

func (st *AclState) applyRequestDecline(ch *aclrecordproto.AclAccountRequestDecline, recordId string, authorIdentity crypto.PubKey) error {
	err := st.contentValidator.ValidateRequestDecline(ch, authorIdentity)
	if err != nil {
		return err
	}
	delete(st.pendingRequests, mapKeyFromPubKey(st.requestRecords[ch.RequestRecordId].RequestIdentity))
	delete(st.requestRecords, ch.RequestRecordId)
	return nil
}

func (st *AclState) applyRequestRemove(ch *aclrecordproto.AclAccountRequestRemove, recordId string, authorIdentity crypto.PubKey) error {
	err := st.contentValidator.ValidateRequestRemove(ch, authorIdentity)
	if err != nil {
		return err
	}
	st.pendingRequests[mapKeyFromPubKey(authorIdentity)] = RequestTypeRemove
	return nil
}

func (st *AclState) applyAccountRemove(ch *aclrecordproto.AclAccountRemove, recordId string, authorIdentity crypto.PubKey) error {
	err := st.contentValidator.ValidateAccountRemove(ch, authorIdentity)
	if err != nil {
		return err
	}
	for _, rawIdentity := range ch.Identities {
		identity, err := st.keyStore.PubKeyFromProto(rawIdentity)
		if err != nil {
			return err
		}
		idKey := mapKeyFromPubKey(identity)
		delete(st.userStates, idKey)
		delete(st.pendingRequests, idKey)
	}
	return st.updateReadKey(ch.AccountKeys, recordId)
}

func (st *AclState) applyReadKeyChange(ch *aclrecordproto.AclReadKeyChange, recordId string, authorIdentity crypto.PubKey) error {
	err := st.contentValidator.ValidateReadKeyChange(ch, authorIdentity)
	if err != nil {
		return err
	}
	return st.updateReadKey(ch.AccountKeys, recordId)
}

func (st *AclState) updateReadKey(keys []*aclrecordproto.AclEncryptedReadKey, recordId string) error {
	for _, accKey := range keys {
		identity, _ := st.keyStore.PubKeyFromProto(accKey.Identity)
		if st.pubKey.Equals(identity) {
			res, err := st.decryptReadKey(accKey.EncryptedReadKey)
			if err != nil {
				return err
			}
			st.userReadKeys[recordId] = res
		}
	}
	st.currentReadKeyId = recordId
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

func (st *AclState) UserStates() map[string]AclUserState {
	return st.userStates
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
