package list

import (
	"errors"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/crypto"
)

var log = logger.NewNamedSugared("common.commonspace.acllist")

var (
	ErrNoSuchAccount             = errors.New("no such account")
	ErrPendingRequest            = errors.New("already exists pending request")
	ErrUnexpectedContentType     = errors.New("unexpected content type")
	ErrIncorrectIdentity         = errors.New("incorrect identity")
	ErrIncorrectInviteKey        = errors.New("incorrect invite key")
	ErrFailedToDecrypt           = errors.New("failed to decrypt key")
	ErrNoMetadataKey             = errors.New("no metadata key")
	ErrNoSuchRecord              = errors.New("no such record")
	ErrNoSuchRequest             = errors.New("no such request")
	ErrNoSuchInvite              = errors.New("no such invite")
	ErrInsufficientPermissions   = errors.New("insufficient permissions")
	ErrIsOwner                   = errors.New("can't be made by owner")
	ErrIncorrectNumberOfAccounts = errors.New("incorrect number of accounts")
	ErrDuplicateAccounts         = errors.New("duplicate accounts")
	ErrNoReadKey                 = errors.New("no read key")
	ErrIncorrectReadKey          = errors.New("incorrect read key")
	ErrInvalidSignature          = errors.New("signature is invalid")
	ErrIncorrectRoot             = errors.New("incorrect root")
	ErrIncorrectRecordSequence   = errors.New("incorrect prev id of a record")
	ErrMetadataTooLarge          = errors.New("metadata size too large")
)

const MaxMetadataLen = 1024

type UserPermissionPair struct {
	Identity   crypto.PubKey
	Permission aclrecordproto.AclUserPermissions
}

type AclKeys struct {
	ReadKey         crypto.SymKey
	MetadataPrivKey crypto.PrivKey
	MetadataPubKey  crypto.PubKey
}

type AclState struct {
	id               string
	currentReadKeyId string
	// keys represent current keys of the acl
	keys map[string]AclKeys
	// accountStates is a map pubKey -> state which defines current account state
	accountStates map[string]AclAccountState
	// statesAtRecord is a map recordId -> state which define account state at particular record
	//  probably this can grow rather large at some point, so we can maybe optimise later to have:
	//  - map pubKey -> []recordIds (where recordIds is an array where such identity permissions were changed)
	statesAtRecord map[string][]AclAccountState
	// inviteKeys is a map recordId -> invite
	inviteKeys map[string]crypto.PubKey
	// requestRecords is a map recordId -> RequestRecord
	requestRecords map[string]RequestRecord
	// pendingRequests is a map pubKey -> recordId
	pendingRequests map[string]string
	// readKeyChanges is a list of records containing read key changes
	readKeyChanges []string
	key            crypto.PrivKey
	pubKey         crypto.PubKey
	keyStore       crypto.KeyStorage

	lastRecordId     string
	contentValidator ContentValidator
	list             AclList
}

func newAclStateWithKeys(
	rootRecord *AclRecord,
	key crypto.PrivKey) (st *AclState, err error) {
	st = &AclState{
		id:              rootRecord.Id,
		key:             key,
		pubKey:          key.GetPublic(),
		keys:            make(map[string]AclKeys),
		accountStates:   make(map[string]AclAccountState),
		statesAtRecord:  make(map[string][]AclAccountState),
		inviteKeys:      make(map[string]crypto.PubKey),
		requestRecords:  make(map[string]RequestRecord),
		pendingRequests: make(map[string]string),
		keyStore:        crypto.NewKeyStorage(),
	}
	st.contentValidator = &contentValidator{
		keyStore: st.keyStore,
		aclState: st,
	}
	err = st.applyRoot(rootRecord)
	if err != nil {
		return
	}
	st.statesAtRecord[rootRecord.Id] = []AclAccountState{
		st.accountStates[mapKeyFromPubKey(rootRecord.Identity)],
	}
	return st, nil
}

func newAclState(rootRecord *AclRecord) (st *AclState, err error) {
	st = &AclState{
		id:              rootRecord.Id,
		keys:            make(map[string]AclKeys),
		accountStates:   make(map[string]AclAccountState),
		statesAtRecord:  make(map[string][]AclAccountState),
		inviteKeys:      make(map[string]crypto.PubKey),
		requestRecords:  make(map[string]RequestRecord),
		pendingRequests: make(map[string]string),
		keyStore:        crypto.NewKeyStorage(),
	}
	st.contentValidator = &contentValidator{
		keyStore: st.keyStore,
		aclState: st,
	}
	err = st.applyRoot(rootRecord)
	if err != nil {
		return
	}
	st.statesAtRecord[rootRecord.Id] = []AclAccountState{
		st.accountStates[mapKeyFromPubKey(rootRecord.Identity)],
	}
	return st, nil
}

func (st *AclState) Validator() ContentValidator {
	return st.contentValidator
}

func (st *AclState) CurrentReadKeyId() string {
	return st.readKeyChanges[len(st.readKeyChanges)-1]
}

func (st *AclState) AccountKey() crypto.PrivKey {
	return st.key
}

func (st *AclState) CurrentReadKey() (crypto.SymKey, error) {
	curKeys, exists := st.keys[st.CurrentReadKeyId()]
	if !exists {
		return nil, ErrNoReadKey
	}
	return curKeys.ReadKey, nil
}

func (st *AclState) CurrentMetadataKey() (crypto.PubKey, error) {
	curKeys, exists := st.keys[st.CurrentReadKeyId()]
	if !exists {
		return nil, ErrNoMetadataKey
	}
	return curKeys.MetadataPubKey, nil
}

func (st *AclState) Keys() map[string]AclKeys {
	return st.keys
}

func (st *AclState) StateAtRecord(id string, pubKey crypto.PubKey) (AclAccountState, error) {
	accountState, ok := st.statesAtRecord[id]
	if !ok {
		log.Errorf("missing record at id %s", id)
		return AclAccountState{}, ErrNoSuchRecord
	}

	for _, perm := range accountState {
		if perm.PubKey.Equals(pubKey) {
			return perm, nil
		}
	}
	return AclAccountState{}, ErrNoSuchAccount
}

func (st *AclState) CurrentStates() []AclAccountState {
	var res []AclAccountState
	for _, state := range st.accountStates {
		res = append(res, state)
	}
	return res
}

type AclAccountDiff struct {
	Added   []AclAccountState
	Removed []AclAccountState
	Changed []AclAccountState
}

func (st *AclState) ChangedStates(first, last string) (diff AclAccountDiff, err error) {
	firstStates, ok := st.statesAtRecord[first]
	if !ok {
		log.Errorf("missing record at id %s", first)
		return
	}
	lastStates, ok := st.statesAtRecord[last]
	if !ok {
		log.Errorf("missing record at id %s", last)
		return
	}
	findFunc := func(states []AclAccountState, state AclAccountState) int {
		for idx, st := range states {
			if st.PubKey.Equals(state.PubKey) {
				return idx
			}
		}
		return -1
	}
	for _, state := range lastStates {
		idx := findFunc(firstStates, state)
		if idx == -1 {
			diff.Added = append(diff.Added, state)
			continue
		}
		if state.Permissions != firstStates[idx].Permissions {
			diff.Changed = append(diff.Changed, state)
		}
	}
	for _, state := range firstStates {
		idx := findFunc(lastStates, state)
		if idx == -1 {
			diff.Removed = append(diff.Removed, state)
		}
	}

	return diff, nil
}

func (st *AclState) applyRecord(record *AclRecord) (err error) {
	if st.lastRecordId != record.PrevId {
		err = ErrIncorrectRecordSequence
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
	var states []AclAccountState
	for _, state := range st.accountStates {
		states = append(states, state)
	}
	st.statesAtRecord[record.Id] = states
	st.lastRecordId = record.Id
	return
}

func (st *AclState) applyRoot(record *AclRecord) (err error) {
	root, ok := record.Model.(*aclrecordproto.AclRoot)
	if !ok {
		return ErrIncorrectRoot
	}
	if root.EncryptedReadKey != nil {
		mkPubKey, err := st.keyStore.PubKeyFromProto(root.MetadataPubKey)
		if err != nil {
			return err
		}
		st.keys[record.Id] = AclKeys{MetadataPubKey: mkPubKey}
	} else {
		// this should be a derived acl
		st.keys[record.Id] = AclKeys{}
	}
	if st.key != nil && st.pubKey.Equals(record.Identity) {
		err = st.saveKeysFromRoot(record.Id, root)
		if err != nil {
			return
		}
	}
	// adding an account to the list
	accountState := AclAccountState{
		PubKey:          record.Identity,
		Permissions:     AclPermissions(aclrecordproto.AclUserPermissions_Owner),
		KeyRecordId:     record.Id,
		RequestMetadata: root.EncryptedOwnerMetadata,
	}
	st.readKeyChanges = []string{record.Id}
	st.accountStates[mapKeyFromPubKey(record.Identity)] = accountState
	st.lastRecordId = record.Id
	return
}

func (st *AclState) saveKeysFromRoot(id string, root *aclrecordproto.AclRoot) (err error) {
	aclKeys := st.keys[id]
	if root.EncryptedReadKey == nil {
		readKey, err := st.deriveKey()
		if err != nil {
			return err
		}
		aclKeys.ReadKey = readKey
	} else {
		readKey, err := st.unmarshallDecryptReadKey(root.EncryptedReadKey, st.key.Decrypt)
		if err != nil {
			return err
		}
		metadataKey, err := st.unmarshallDecryptPrivKey(root.EncryptedMetadataPrivKey, readKey.Decrypt)
		if err != nil {
			return err
		}
		aclKeys.ReadKey = readKey
		aclKeys.MetadataPrivKey = metadataKey
	}
	st.keys[id] = aclKeys
	return
}

func (st *AclState) applyChangeData(record *AclRecord) (err error) {
	model := record.Model.(*aclrecordproto.AclData)
	for _, ch := range model.GetAclContent() {
		if err = st.applyChangeContent(ch, record); err != nil {
			log.Info("error while applying changes; ignore", zap.Error(err))
			return err
		}
	}
	return nil
}

func (st *AclState) applyChangeContent(ch *aclrecordproto.AclContentValue, record *AclRecord) error {
	switch {
	case ch.GetPermissionChange() != nil:
		return st.applyPermissionChange(ch.GetPermissionChange(), record)
	case ch.GetInvite() != nil:
		return st.applyInvite(ch.GetInvite(), record)
	case ch.GetInviteRevoke() != nil:
		return st.applyInviteRevoke(ch.GetInviteRevoke(), record)
	case ch.GetRequestJoin() != nil:
		return st.applyRequestJoin(ch.GetRequestJoin(), record)
	case ch.GetRequestAccept() != nil:
		return st.applyRequestAccept(ch.GetRequestAccept(), record)
	case ch.GetRequestDecline() != nil:
		return st.applyRequestDecline(ch.GetRequestDecline(), record)
	case ch.GetAccountRemove() != nil:
		return st.applyAccountRemove(ch.GetAccountRemove(), record)
	case ch.GetReadKeyChange() != nil:
		return st.applyReadKeyChange(ch.GetReadKeyChange(), record, true)
	case ch.GetAccountRequestRemove() != nil:
		return st.applyRequestRemove(ch.GetAccountRequestRemove(), record)
	default:
		return ErrUnexpectedContentType
	}
}

func (st *AclState) applyPermissionChange(ch *aclrecordproto.AclAccountPermissionChange, record *AclRecord) error {
	chIdentity, err := st.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return err
	}
	err = st.contentValidator.ValidatePermissionChange(ch, record.Identity)
	if err != nil {
		return err
	}
	stringKey := mapKeyFromPubKey(chIdentity)
	state, _ := st.accountStates[stringKey]
	state.Permissions = AclPermissions(ch.Permissions)
	st.accountStates[stringKey] = state
	return nil
}

func (st *AclState) applyInvite(ch *aclrecordproto.AclAccountInvite, record *AclRecord) error {
	inviteKey, err := st.keyStore.PubKeyFromProto(ch.InviteKey)
	if err != nil {
		return err
	}
	err = st.contentValidator.ValidateInvite(ch, record.Identity)
	if err != nil {
		return err
	}
	st.inviteKeys[record.Id] = inviteKey
	return nil
}

func (st *AclState) applyInviteRevoke(ch *aclrecordproto.AclAccountInviteRevoke, record *AclRecord) error {
	err := st.contentValidator.ValidateInviteRevoke(ch, record.Identity)
	if err != nil {
		return err
	}
	delete(st.inviteKeys, ch.InviteRecordId)
	return nil
}

func (st *AclState) applyRequestJoin(ch *aclrecordproto.AclAccountRequestJoin, record *AclRecord) error {
	err := st.contentValidator.ValidateRequestJoin(ch, record.Identity)
	if err != nil {
		return err
	}
	st.pendingRequests[mapKeyFromPubKey(record.Identity)] = record.Id
	st.requestRecords[record.Id] = RequestRecord{
		RequestIdentity: record.Identity,
		RequestMetadata: ch.Metadata,
		KeyRecordId:     st.CurrentReadKeyId(),
		Type:            RequestTypeJoin,
	}
	return nil
}

func (st *AclState) applyRequestAccept(ch *aclrecordproto.AclAccountRequestAccept, record *AclRecord) error {
	err := st.contentValidator.ValidateRequestAccept(ch, record.Identity)
	if err != nil {
		return err
	}
	acceptIdentity, err := st.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return err
	}
	requestRecord, _ := st.requestRecords[ch.RequestRecordId]
	st.accountStates[mapKeyFromPubKey(acceptIdentity)] = AclAccountState{
		PubKey:          acceptIdentity,
		Permissions:     AclPermissions(ch.Permissions),
		RequestMetadata: requestRecord.RequestMetadata,
		KeyRecordId:     requestRecord.KeyRecordId,
	}
	delete(st.pendingRequests, mapKeyFromPubKey(st.requestRecords[ch.RequestRecordId].RequestIdentity))
	if !st.pubKey.Equals(acceptIdentity) {
		return nil
	}
	iterReadKey, err := st.unmarshallDecryptReadKey(ch.EncryptedReadKey, st.key.Decrypt)
	if err != nil {
		return err
	}
	for idx := len(st.readKeyChanges) - 1; idx >= 0; idx-- {
		recId := st.readKeyChanges[idx]
		rec, err := st.list.Get(recId)
		if err != nil {
			return err
		}
		// if this is a first key change
		if recId == st.id {
			ch := rec.Model.(*aclrecordproto.AclRoot)
			metadataKey, err := st.unmarshallDecryptPrivKey(ch.EncryptedMetadataPrivKey, iterReadKey.Decrypt)
			if err != nil {
				return err
			}
			aclKeys := st.keys[recId]
			aclKeys.ReadKey = iterReadKey
			aclKeys.MetadataPrivKey = metadataKey
			st.keys[recId] = aclKeys
			break
		}
		model := rec.Model.(*aclrecordproto.AclData)
		if len(model.GetAclContent()) != 1 {
			return ErrIncorrectReadKey
		}
		ch := model.GetAclContent()[0]
		var readKeyChange *aclrecordproto.AclReadKeyChange
		switch {
		case ch.GetReadKeyChange() != nil:
			readKeyChange = ch.GetReadKeyChange()
		case ch.GetAccountRemove() != nil:
			readKeyChange = ch.GetAccountRemove().GetReadKeyChange()
		}
		oldReadKey, err := st.unmarshallDecryptReadKey(readKeyChange.EncryptedOldReadKey, iterReadKey.Decrypt)
		if err != nil {
			return err
		}
		metadataKey, err := st.unmarshallDecryptPrivKey(readKeyChange.EncryptedMetadataPrivKey, iterReadKey.Decrypt)
		if err != nil {
			return err
		}
		aclKeys := st.keys[recId]
		aclKeys.ReadKey = iterReadKey
		aclKeys.MetadataPrivKey = metadataKey
		st.keys[recId] = aclKeys
		iterReadKey = oldReadKey
	}
	return nil
}

func (st *AclState) applyRequestDecline(ch *aclrecordproto.AclAccountRequestDecline, record *AclRecord) error {
	err := st.contentValidator.ValidateRequestDecline(ch, record.Identity)
	if err != nil {
		return err
	}
	delete(st.pendingRequests, mapKeyFromPubKey(st.requestRecords[ch.RequestRecordId].RequestIdentity))
	delete(st.requestRecords, ch.RequestRecordId)
	return nil
}

func (st *AclState) applyRequestRemove(ch *aclrecordproto.AclAccountRequestRemove, record *AclRecord) error {
	err := st.contentValidator.ValidateRequestRemove(ch, record.Identity)
	if err != nil {
		return err
	}
	st.requestRecords[record.Id] = RequestRecord{
		RequestIdentity: record.Identity,
		Type:            RequestTypeRemove,
	}
	st.pendingRequests[mapKeyFromPubKey(record.Identity)] = record.Id
	return nil
}

func (st *AclState) applyAccountRemove(ch *aclrecordproto.AclAccountRemove, record *AclRecord) error {
	err := st.contentValidator.ValidateAccountRemove(ch, record.Identity)
	if err != nil {
		return err
	}
	for _, rawIdentity := range ch.Identities {
		identity, err := st.keyStore.PubKeyFromProto(rawIdentity)
		if err != nil {
			return err
		}
		idKey := mapKeyFromPubKey(identity)
		delete(st.accountStates, idKey)
		delete(st.pendingRequests, idKey)
	}
	return st.applyReadKeyChange(ch.ReadKeyChange, record, false)
}

func (st *AclState) applyReadKeyChange(ch *aclrecordproto.AclReadKeyChange, record *AclRecord, validate bool) error {
	if validate {
		err := st.contentValidator.ValidateReadKeyChange(ch, record.Identity)
		if err != nil {
			return err
		}
	}
	st.readKeyChanges = append(st.readKeyChanges, record.Id)
	mkPubKey, err := st.keyStore.PubKeyFromProto(ch.MetadataPubKey)
	if err != nil {
		return err
	}
	aclKeys := AclKeys{
		MetadataPubKey: mkPubKey,
	}
	for _, accKey := range ch.AccountKeys {
		identity, _ := st.keyStore.PubKeyFromProto(accKey.Identity)
		if st.pubKey.Equals(identity) {
			res, err := st.unmarshallDecryptReadKey(accKey.EncryptedReadKey, st.key.Decrypt)
			if err != nil {
				return err
			}
			aclKeys.ReadKey = res
		}
	}
	if aclKeys.ReadKey != nil {
		metadataKey, err := st.unmarshallDecryptPrivKey(ch.EncryptedMetadataPrivKey, aclKeys.ReadKey.Decrypt)
		if err != nil {
			return err
		}
		aclKeys.MetadataPrivKey = metadataKey
	}
	st.keys[record.Id] = aclKeys
	return nil
}

func (st *AclState) unmarshallDecryptReadKey(msg []byte, decryptor func(msg []byte) ([]byte, error)) (crypto.SymKey, error) {
	decrypted, err := decryptor(msg)
	if err != nil {
		return nil, ErrFailedToDecrypt
	}
	key, err := crypto.UnmarshallAESKeyProto(decrypted)
	if err != nil {
		return nil, ErrFailedToDecrypt
	}
	return key, nil
}

func (st *AclState) unmarshallDecryptPrivKey(msg []byte, decryptor func(msg []byte) ([]byte, error)) (crypto.PrivKey, error) {
	decrypted, err := decryptor(msg)
	if err != nil {
		return nil, ErrFailedToDecrypt
	}
	key, err := crypto.UnmarshalEd25519PrivateKeyProto(decrypted)
	if err != nil {
		return nil, ErrFailedToDecrypt
	}
	return key, nil
}

func (st *AclState) GetMetadata(identity crypto.PubKey, decrypt bool) (res []byte, err error) {
	state, exists := st.accountStates[mapKeyFromPubKey(identity)]
	if !exists {
		return nil, ErrNoSuchAccount
	}
	if !decrypt {
		return state.RequestMetadata, nil
	}
	aclKeys := st.keys[state.KeyRecordId]
	if aclKeys.MetadataPrivKey == nil {
		return nil, ErrFailedToDecrypt
	}
	return aclKeys.MetadataPrivKey.Decrypt(state.RequestMetadata)
}

func (st *AclState) Permissions(identity crypto.PubKey) AclPermissions {
	state, exists := st.accountStates[mapKeyFromPubKey(identity)]
	if !exists {
		return AclPermissions(aclrecordproto.AclUserPermissions_None)
	}
	return state.Permissions
}

func (st *AclState) JoinRecords(decrypt bool) (records []RequestRecord, err error) {
	for _, recId := range st.pendingRequests {
		rec := st.requestRecords[recId]
		if rec.Type == RequestTypeJoin {
			if decrypt {
				aclKeys := st.keys[rec.KeyRecordId]
				if aclKeys.MetadataPrivKey == nil {
					return nil, ErrFailedToDecrypt
				}
				res, err := aclKeys.MetadataPrivKey.Decrypt(rec.RequestMetadata)
				if err != nil {
					return nil, err
				}
				rec.RequestMetadata = res
			}
			records = append(records, rec)
		}
	}
	return
}

func (st *AclState) RemoveRecords() (records []RequestRecord) {
	for _, recId := range st.pendingRequests {
		rec := st.requestRecords[recId]
		if rec.Type == RequestTypeRemove {
			records = append(records, rec)
		}
	}
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
