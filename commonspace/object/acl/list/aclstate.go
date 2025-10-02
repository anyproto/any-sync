package list

import (
	"errors"

	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
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
	ErrDuplicateInvites          = errors.New("duplicate invites")
	ErrIsOwner                   = errors.New("can't be made by owner")
	ErrIncorrectNumberOfAccounts = errors.New("incorrect number of accounts")
	ErrDuplicateAccounts         = errors.New("duplicate accounts")
	ErrNoReadKey                 = errors.New("no read key")
	ErrIncorrectReadKey          = errors.New("incorrect read key")
	ErrInvalidSignature          = errors.New("signature is invalid")
	ErrIncorrectRoot             = errors.New("incorrect root")
	ErrIncorrectRecordSequence   = errors.New("incorrect prev id of a record")
	ErrMetadataTooLarge          = errors.New("metadata size too large")
	ErrOwnerNotFound             = errors.New("owner not found")
	ErrAddRecordOneToOne         = errors.New("adding a record to one-to-one space is forbidden")
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

	oldEncryptedReadKey []byte
	encMetadatKey       []byte
}

type Invite struct {
	Key          crypto.PubKey
	Type         aclrecordproto.AclInviteType
	Permissions  AclPermissions
	Id           string
	encryptedKey []byte
}

type AclState struct {
	id string
	// keys represent current keys of the acl
	keys map[string]AclKeys
	// accountStates is a map pubKey -> state which defines current account state
	accountStates map[string]AccountState
	// inviteKeys is a map recordId -> invite
	invites map[string]Invite
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
	list             *aclList

	isOneToOne bool
}

func newAclStateWithKeys(
	rootRecord *AclRecord,
	key crypto.PrivKey,
	verifier recordverifier.AcceptorVerifier) (st *AclState, err error) {
	st = &AclState{
		id:              rootRecord.Id,
		key:             key,
		pubKey:          key.GetPublic(),
		keys:            make(map[string]AclKeys),
		accountStates:   make(map[string]AccountState),
		invites:         make(map[string]Invite),
		requestRecords:  make(map[string]RequestRecord),
		pendingRequests: make(map[string]string),
		keyStore:        crypto.NewKeyStorage(),
	}
	st.contentValidator = newContentValidator(st.keyStore, st, verifier)
	err = st.applyRoot(rootRecord)
	if err != nil {
		return
	}
	return st, nil
}

func newAclState(rootRecord *AclRecord, verifier recordverifier.AcceptorVerifier) (st *AclState, err error) {
	st = &AclState{
		id:              rootRecord.Id,
		keys:            make(map[string]AclKeys),
		accountStates:   make(map[string]AccountState),
		invites:         make(map[string]Invite),
		requestRecords:  make(map[string]RequestRecord),
		pendingRequests: make(map[string]string),
		keyStore:        crypto.NewKeyStorage(),
	}
	st.contentValidator = newContentValidator(st.keyStore, st, verifier)
	err = st.applyRoot(rootRecord)
	if err != nil {
		return
	}
	return st, nil
}

func (st *AclState) Identity() crypto.PubKey {
	return st.pubKey
}

func (st *AclState) Validator() ContentValidator {
	return st.contentValidator
}

func (st *AclState) CurrentReadKeyId() string {
	return st.readKeyChanges[len(st.readKeyChanges)-1]
}

func (st *AclState) ReadKeyForAclId(id string) (string, error) {
	recIdx, ok := st.list.indexes[id]
	if !ok {
		return "", ErrNoSuchRecord
	}
	for i := len(st.readKeyChanges) - 1; i >= 0; i-- {
		recId := st.readKeyChanges[i]
		if recIdx >= st.list.indexes[recId] {
			return recId, nil
		}
	}
	return "", ErrNoSuchRecord
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

func (st *AclState) FirstMetadataKey() (crypto.PrivKey, error) {
	if firstKey, ok := st.keys[st.id]; ok && firstKey.MetadataPrivKey != nil {
		return firstKey.MetadataPrivKey, nil
	}
	return nil, ErrNoMetadataKey
}

func (st *AclState) Keys() map[string]AclKeys {
	return st.keys
}

func (st *AclState) PermissionsAtRecord(id string, pubKey crypto.PubKey) (AclPermissions, error) {
	if !st.list.HasHead(id) {
		return AclPermissionsNone, ErrNoSuchRecord
	}
	accountState, ok := st.accountStates[mapKeyFromPubKey(pubKey)]
	if !ok {
		return AclPermissionsNone, ErrNoSuchAccount
	}
	perms := closestPermissions(accountState, id, st.list.isAfterNoCheck)
	return perms, nil
}

func (st *AclState) CurrentAccounts() []AccountState {
	var res []AccountState
	for _, state := range st.accountStates {
		res = append(res, state)
	}
	return res
}

func (st *AclState) HadReadPermissions(identity crypto.PubKey) (had bool) {
	state, exists := st.accountStates[mapKeyFromPubKey(identity)]
	if !exists {
		return false
	}
	for _, perm := range state.PermissionChanges {
		if !perm.Permission.NoPermissions() {
			return true
		}
	}
	return false
}

func (st *AclState) Invites(inviteType ...aclrecordproto.AclInviteType) []Invite {
	var invites []Invite
	for _, inv := range st.invites {
		if len(inviteType) > 0 && !slices.Contains(inviteType, inv.Type) {
			continue
		}
		invites = append(invites, inv)
	}
	return invites
}

func (st *AclState) Key() crypto.PrivKey {
	return st.key
}

func (st *AclState) InviteIds() []string {
	var invites []string
	for invId := range st.invites {
		invites = append(invites, invId)
	}
	return invites
}

func (st *AclState) RequestIds() []string {
	var requests []string
	for reqId := range st.requestRecords {
		requests = append(requests, reqId)
	}
	return requests
}

func (st *AclState) DecryptInvite(invitePk crypto.PrivKey) (key crypto.SymKey, err error) {
	if invitePk == nil {
		return nil, ErrNoReadKey
	}
	for _, invite := range st.invites {
		if invite.Key.Equals(invitePk.GetPublic()) {
			res, err := st.unmarshallDecryptReadKey(invite.encryptedKey, invitePk.Decrypt)
			if err != nil {
				return nil, err
			}
			return res, nil
		}
	}
	return nil, ErrNoSuchInvite
}

func (st *AclState) ApplyRecord(record *AclRecord) (err error) {
	if st.IsOneToOne() {
		return ErrAddRecordOneToOne
	}

	if st.lastRecordId != record.PrevId {
		err = ErrIncorrectRecordSequence
		return
	}
	// if the model is not cached
	if record.Model == nil {
		aclData := &aclrecordproto.AclData{}
		err = aclData.UnmarshalVT(record.Data)
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
	st.lastRecordId = record.Id
	return
}

func (st *AclState) IsEmpty() bool {
	users := 0
	for _, acc := range st.CurrentAccounts() {
		if !acc.Permissions.NoPermissions() {
			users++
		}
	}
	return users == 1 && len(st.Invites()) == 0 && len(st.pendingRequests) == 0
}

func (st *AclState) applyRoot(record *AclRecord) (err error) {
	root, ok := record.Model.(*aclrecordproto.AclRoot)
	if !ok {
		return ErrIncorrectRoot
	}

	if root.OneToOneInfo != nil {
		return st.setOneToOneAcl(record.Id, root)
	}

	if root.EncryptedReadKey != nil {
		mkPubKey, err := st.keyStore.PubKeyFromProto(root.MetadataPubKey)
		if err != nil {
			return err
		}
		st.keys[record.Id] = AclKeys{
			MetadataPubKey: mkPubKey,
			encMetadatKey:  root.EncryptedMetadataPrivKey,
		}
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
	accountState := AccountState{
		PubKey:          record.Identity,
		Permissions:     AclPermissions(aclrecordproto.AclUserPermissions_Owner),
		KeyRecordId:     record.Id,
		RequestMetadata: root.EncryptedOwnerMetadata,
		Status:          StatusActive,
		PermissionChanges: []PermissionChange{
			{
				RecordId:   record.Id,
				Permission: AclPermissionsOwner,
			},
		},
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
			log.Info("error while applying changes", zap.Error(err))
			return err
		}
	}
	return nil
}

func (st *AclState) Copy() *AclState {
	newSt := &AclState{
		id:              st.id,
		key:             st.key,
		pubKey:          st.key.GetPublic(),
		keys:            make(map[string]AclKeys),
		accountStates:   make(map[string]AccountState),
		invites:         make(map[string]Invite),
		requestRecords:  make(map[string]RequestRecord),
		pendingRequests: make(map[string]string),
		keyStore:        st.keyStore,
	}
	for k, v := range st.keys {
		newSt.keys[k] = v
	}
	for k, v := range st.accountStates {
		var permChanges []PermissionChange
		permChanges = append(permChanges, v.PermissionChanges...)
		accState := v
		accState.PermissionChanges = permChanges
		newSt.accountStates[k] = accState
	}
	for k, v := range st.invites {
		newSt.invites[k] = v
	}
	for k, v := range st.requestRecords {
		newSt.requestRecords[k] = v
	}
	for k, v := range st.pendingRequests {
		newSt.pendingRequests[k] = v
	}
	newSt.readKeyChanges = append(newSt.readKeyChanges, st.readKeyChanges...)
	newSt.list = st.list
	newSt.lastRecordId = st.lastRecordId
	newSt.contentValidator = newContentValidator(newSt.keyStore, newSt, st.list.verifier)
	return newSt
}

func (st *AclState) applyChangeContent(ch *aclrecordproto.AclContentValue, record *AclRecord) error {
	switch {
	case ch.GetOwnershipChange() != nil:
		return st.applyOwnershipChange(ch.GetOwnershipChange(), record)
	case ch.GetInviteChange() != nil:
		return st.applyInviteChange(ch.GetInviteChange(), record)
	case ch.GetInviteJoin() != nil:
		return st.applyInviteJoin(ch.GetInviteJoin(), record)
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
	case ch.GetRequestCancel() != nil:
		return st.applyRequestCancel(ch.GetRequestCancel(), record)
	case ch.GetAccountRemove() != nil:
		return st.applyAccountRemove(ch.GetAccountRemove(), record)
	case ch.GetReadKeyChange() != nil:
		return st.applyReadKeyChange(ch.GetReadKeyChange(), record, true)
	case ch.GetAccountRequestRemove() != nil:
		return st.applyRequestRemove(ch.GetAccountRequestRemove(), record)
	case ch.GetAccountsAdd() != nil:
		return st.applyAccountsAdd(ch.GetAccountsAdd(), record)
	case ch.GetPermissionChanges() != nil:
		return st.applyPermissionChanges(ch.GetPermissionChanges(), record)
	default:
		log.Errorf("got unexpected content type: %s", record.Id)
		return nil
	}
}

func (st *AclState) applyOwnershipChange(ch *aclrecordproto.AclOwnershipChange, record *AclRecord) (err error) {
	err = st.contentValidator.ValidateOwnershipChange(ch, record.Identity)
	if err != nil {
		return err
	}
	newOwnerIdentity, err := st.keyStore.PubKeyFromProto(ch.NewOwnerIdentity)
	if err != nil {
		return err
	}
	var (
		oldOwnerKey = mapKeyFromPubKey(record.Identity)
		newOwnerKey = mapKeyFromPubKey(newOwnerIdentity)
	)
	st.updatePermissions(oldOwnerKey, ch.OldOwnerPermissions, record)
	st.updatePermissions(newOwnerKey, aclrecordproto.AclUserPermissions_Owner, record)
	return nil
}

func (st *AclState) applyPermissionChanges(ch *aclrecordproto.AclAccountPermissionChanges, record *AclRecord) (err error) {
	for _, ch := range ch.Changes {
		err := st.applyPermissionChange(ch, record)
		if err != nil {
			return err
		}
	}
	return nil
}

func (st *AclState) applyInviteChange(ch *aclrecordproto.AclAccountInviteChange, record *AclRecord) (err error) {
	err = st.contentValidator.ValidateInviteChange(ch, record.Identity)
	if err != nil {
		return err
	}
	invite := st.invites[ch.InviteRecordId]
	invite.Permissions = AclPermissions(ch.Permissions)
	st.invites[ch.InviteRecordId] = invite
	return nil
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
	st.updatePermissions(stringKey, ch.Permissions, record)
	return nil
}

func (st *AclState) updatePermissions(identityKey string, permissions aclrecordproto.AclUserPermissions, record *AclRecord) {
	state, _ := st.accountStates[identityKey]
	state.Permissions = AclPermissions(permissions)
	state.PermissionChanges = append(state.PermissionChanges, PermissionChange{
		RecordId:   record.Id,
		Permission: state.Permissions,
	})
	st.accountStates[identityKey] = state
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
	st.invites[record.Id] = Invite{
		Key:          inviteKey,
		Id:           record.Id,
		Type:         ch.InviteType,
		Permissions:  AclPermissions(ch.Permissions),
		encryptedKey: ch.EncryptedReadKey,
	}
	return nil
}

func (st *AclState) applyInviteRevoke(ch *aclrecordproto.AclAccountInviteRevoke, record *AclRecord) error {
	err := st.contentValidator.ValidateInviteRevoke(ch, record.Identity)
	if err != nil {
		return err
	}
	delete(st.invites, ch.InviteRecordId)
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
		RecordId:        record.Id,
		Type:            RequestTypeJoin,
	}
	pKeyString := mapKeyFromPubKey(record.Identity)
	state, exists := st.accountStates[pKeyString]
	if !exists {
		st.accountStates[mapKeyFromPubKey(record.Identity)] = AccountState{
			PubKey:          record.Identity,
			Permissions:     AclPermissionsNone,
			Status:          StatusJoining,
			RequestMetadata: ch.Metadata,
			KeyRecordId:     st.CurrentReadKeyId(),
		}
	} else {
		st.accountStates[mapKeyFromPubKey(record.Identity)] = AccountState{
			PubKey:            record.Identity,
			Permissions:       AclPermissionsNone,
			Status:            StatusJoining,
			RequestMetadata:   ch.Metadata,
			KeyRecordId:       st.CurrentReadKeyId(),
			PermissionChanges: state.PermissionChanges,
		}
	}
	return nil
}

func (st *AclState) applyAccountsAdd(ch *aclrecordproto.AclAccountsAdd, record *AclRecord) error {
	err := st.contentValidator.ValidateAccountsAdd(ch, record.Identity)
	if err != nil {
		return err
	}
	for _, acc := range ch.Additions {
		identity, err := st.keyStore.PubKeyFromProto(acc.Identity)
		if err != nil {
			return err
		}
		st.accountStates[mapKeyFromPubKey(identity)] = AccountState{
			PubKey:          identity,
			Permissions:     AclPermissions(acc.Permissions),
			Status:          StatusActive,
			RequestMetadata: acc.Metadata,
			KeyRecordId:     st.CurrentReadKeyId(),
			PermissionChanges: []PermissionChange{
				{
					Permission: AclPermissions(acc.Permissions),
					RecordId:   record.Id,
				},
			},
		}
		if !st.pubKey.Equals(identity) {
			continue
		}
		err = st.unpackAllKeys(acc.EncryptedReadKey)
		if err != nil {
			return err
		}
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
	pKeyString := mapKeyFromPubKey(acceptIdentity)
	state, exists := st.accountStates[pKeyString]
	permissions := AclPermissions(ch.Permissions)
	permissionChanges := []PermissionChange{{Permission: permissions, RecordId: record.Id}}
	if exists {
		permissionChanges = append(state.PermissionChanges, permissionChanges[0])
	}

	st.accountStates[pKeyString] = AccountState{
		PubKey:            acceptIdentity,
		Permissions:       permissions,
		RequestMetadata:   requestRecord.RequestMetadata,
		KeyRecordId:       requestRecord.KeyRecordId,
		Status:            StatusActive,
		PermissionChanges: permissionChanges,
	}
	delete(st.pendingRequests, mapKeyFromPubKey(st.requestRecords[ch.RequestRecordId].RequestIdentity))
	delete(st.requestRecords, ch.RequestRecordId)
	if !st.pubKey.Equals(acceptIdentity) {
		return nil
	}
	return st.unpackAllKeys(ch.EncryptedReadKey)
}

func (st *AclState) applyInviteJoin(ch *aclrecordproto.AclAccountInviteJoin, record *AclRecord) error {
	err := st.contentValidator.ValidateInviteJoin(ch, record.Identity)
	if err != nil {
		return err
	}
	identity, err := st.keyStore.PubKeyFromProto(ch.Identity)
	if err != nil {
		return err
	}
	inviteRecord, _ := st.invites[ch.InviteRecordId]
	permissions := AclPermissions(ch.Permissions)
	if permissions.NoPermissions() {
		permissions = inviteRecord.Permissions
	}
	pKeyString := mapKeyFromPubKey(identity)
	state, exists := st.accountStates[pKeyString]
	permissionChanges := []PermissionChange{{Permission: permissions, RecordId: record.Id}}
	if exists {
		permissionChanges = append(state.PermissionChanges, permissionChanges[0])
	}
	st.accountStates[pKeyString] = AccountState{
		PubKey:            identity,
		Permissions:       permissions,
		RequestMetadata:   ch.Metadata,
		KeyRecordId:       st.CurrentReadKeyId(),
		Status:            StatusActive,
		PermissionChanges: permissionChanges,
	}
	for _, rec := range st.requestRecords {
		if rec.RequestIdentity.Equals(identity) {
			delete(st.pendingRequests, mapKeyFromPubKey(rec.RequestIdentity))
			delete(st.requestRecords, rec.RecordId)
			break
		}
	}
	if st.pubKey.Equals(identity) {
		return st.unpackAllKeys(ch.EncryptedReadKey)
	}
	return nil
}

func (st *AclState) unpackAllKeys(rk []byte) error {
	iterReadKey, err := st.unmarshallDecryptReadKey(rk, st.key.Decrypt)
	if err != nil {
		return err
	}
	for idx := len(st.readKeyChanges) - 1; idx >= 0; idx-- {
		recId := st.readKeyChanges[idx]
		keys := st.keys[recId]
		metadataKey, err := st.unmarshallDecryptPrivKey(keys.encMetadatKey, iterReadKey.Decrypt)
		if err != nil {
			return err
		}
		aclKeys := st.keys[recId]
		aclKeys.ReadKey = iterReadKey
		aclKeys.MetadataPrivKey = metadataKey
		st.keys[recId] = aclKeys
		if idx != 0 {
			if keys.oldEncryptedReadKey == nil {
				return ErrIncorrectReadKey
			}
			oldReadKey, err := st.unmarshallDecryptReadKey(keys.oldEncryptedReadKey, iterReadKey.Decrypt)
			if err != nil {
				return err
			}
			iterReadKey = oldReadKey
		}
	}
	return nil
}

func (st *AclState) applyRequestDecline(ch *aclrecordproto.AclAccountRequestDecline, record *AclRecord) error {
	err := st.contentValidator.ValidateRequestDecline(ch, record.Identity)
	if err != nil {
		return err
	}
	pk := mapKeyFromPubKey(st.requestRecords[ch.RequestRecordId].RequestIdentity)
	accSt, exists := st.accountStates[pk]
	if !exists {
		return ErrNoSuchAccount
	}
	accSt.Status = StatusDeclined
	st.accountStates[pk] = accSt
	delete(st.pendingRequests, mapKeyFromPubKey(st.requestRecords[ch.RequestRecordId].RequestIdentity))
	delete(st.requestRecords, ch.RequestRecordId)
	return nil
}

func (st *AclState) applyRequestCancel(ch *aclrecordproto.AclAccountRequestCancel, record *AclRecord) error {
	err := st.contentValidator.ValidateRequestCancel(ch, record.Identity)
	if err != nil {
		return err
	}
	pk := mapKeyFromPubKey(st.requestRecords[ch.RecordId].RequestIdentity)
	accSt, exists := st.accountStates[pk]
	if !exists {
		return ErrNoSuchAccount
	}
	rec := st.requestRecords[ch.RecordId]
	if rec.Type == RequestTypeJoin {
		accSt.Status = StatusCanceled
	} else {
		accSt.Status = StatusActive
	}
	st.accountStates[pk] = accSt
	delete(st.pendingRequests, mapKeyFromPubKey(st.requestRecords[ch.RecordId].RequestIdentity))
	delete(st.requestRecords, ch.RecordId)
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
		RecordId:        record.Id,
	}
	st.pendingRequests[mapKeyFromPubKey(record.Identity)] = record.Id
	pk := mapKeyFromPubKey(record.Identity)
	accSt, exists := st.accountStates[pk]
	if !accSt.Permissions.CanRequestRemove() {
		return ErrInsufficientPermissions
	}
	if !exists {
		return ErrNoSuchAccount
	}
	accSt.Status = StatusRemoving
	st.accountStates[pk] = accSt
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
		accSt, exists := st.accountStates[idKey]
		if !exists {
			return ErrNoSuchAccount
		}
		accSt.Status = StatusRemoved
		accSt.Permissions = AclPermissionsNone
		accSt.PermissionChanges = append(accSt.PermissionChanges, PermissionChange{
			RecordId:   record.Id,
			Permission: AclPermissionsNone,
		})
		st.accountStates[idKey] = accSt
		recId, exists := st.pendingRequests[idKey]
		if exists {
			delete(st.pendingRequests, idKey)
			delete(st.requestRecords, recId)
		}
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
		MetadataPubKey:      mkPubKey,
		oldEncryptedReadKey: ch.EncryptedOldReadKey,
		encMetadatKey:       ch.EncryptedMetadataPrivKey,
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
	for _, encKey := range ch.InviteKeys {
		invKey, err := st.keyStore.PubKeyFromProto(encKey.Identity)
		if err != nil {
			return err
		}
		for key, invite := range st.invites {
			if invite.Key.Equals(invKey) {
				invite.encryptedKey = encKey.EncryptedReadKey
				st.invites[key] = invite
				break
			}
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

func (st *AclState) GetInviteIdByPrivKey(inviteKey crypto.PrivKey) (recId string, err error) {
	for id, inv := range st.invites {
		if inv.Key.Equals(inviteKey.GetPublic()) {
			return id, nil
		}
	}
	return "", ErrNoSuchRecord
}

func (st *AclState) GetMetadata(identity crypto.PubKey, decrypt bool) (res []byte, err error) {
	state, exists := st.accountStates[mapKeyFromPubKey(identity)]
	if !exists {
		return nil, ErrNoSuchAccount
	}
	if !decrypt || st.IsOneToOne() {
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

func (st *AclState) Record(identity crypto.PubKey) (RequestRecord, error) {
	recId, exists := st.pendingRequests[mapKeyFromPubKey(identity)]
	if !exists {
		return RequestRecord{}, ErrNoSuchRecord
	}
	return st.requestRecords[recId], nil
}

func (st *AclState) JoinRecord(identity crypto.PubKey, decrypt bool) (RequestRecord, error) {
	recId, exists := st.pendingRequests[mapKeyFromPubKey(identity)]
	if !exists {
		return RequestRecord{}, ErrNoSuchRecord
	}
	rec := st.requestRecords[recId]
	if decrypt {
		aclKeys := st.keys[rec.KeyRecordId]
		if aclKeys.MetadataPrivKey == nil {
			return RequestRecord{}, ErrFailedToDecrypt
		}
		res, err := aclKeys.MetadataPrivKey.Decrypt(rec.RequestMetadata)
		if err != nil {
			return RequestRecord{}, err
		}
		rec.RequestMetadata = res
	}
	return rec, nil
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

func (st *AclState) OwnerPubKey() (ownerIdentity crypto.PubKey, err error) {
	for _, aState := range st.accountStates {
		if aState.Permissions.IsOwner() {
			return aState.PubKey, nil
		}
	}
	return nil, ErrOwnerNotFound
}

func (st *AclState) IsOneToOne() bool {
	return st.isOneToOne
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

func closestPermissions(accountState AccountState, recordId string, isAfter func(first, second string) bool) AclPermissions {
	if len(accountState.PermissionChanges) == 0 {
		return AclPermissionsNone
	}
	permChanges := accountState.PermissionChanges
	for i := len(permChanges) - 1; i >= 0; i-- {
		if isAfter(recordId, permChanges[i].RecordId) {
			return permChanges[i].Permission
		}
	}
	return AclPermissionsNone
}
