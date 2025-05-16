package list

import (
	"errors"
	"time"

	"github.com/anyproto/protobuf/proto"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
)

type RootContent struct {
	PrivKey   crypto.PrivKey
	MasterKey crypto.PrivKey
	SpaceId   string
	Change    ReadKeyChangePayload
	Metadata  []byte
}

type RequestJoinPayload struct {
	InviteKey crypto.PrivKey
	Metadata  []byte
}

type InviteJoinPayload struct {
	InviteKey crypto.PrivKey
	Metadata  []byte
}

type ReadKeyChangePayload struct {
	MetadataKey crypto.PrivKey
	ReadKey     crypto.SymKey
}

type RequestAcceptPayload struct {
	RequestRecordId string
	Permissions     AclPermissions
}

type PermissionChangePayload struct {
	Identity    crypto.PubKey
	Permissions AclPermissions
}

type InviteChangePayload struct {
	IniviteRecordId string
	Permissions     AclPermissions
}

type PermissionChangesPayload struct {
	Changes []PermissionChangePayload
}

type AccountsAddPayload struct {
	Additions []AccountAdd
}

type AccountAdd struct {
	Identity    crypto.PubKey
	Permissions AclPermissions
	Metadata    []byte
}

type NewInvites struct {
	Permissions AclPermissions
}

type BatchRequestPayload struct {
	Additions     []AccountAdd
	Changes       []PermissionChangePayload
	Removals      AccountRemovePayload
	Approvals     []RequestAcceptPayload
	Declines      []string
	InviteRevokes []string
	InviteChanges []InviteChangePayload
	NewInvites    []AclPermissions
}

type AccountRemovePayload struct {
	Identities []crypto.PubKey
	Change     ReadKeyChangePayload
}

type InviteResult struct {
	InviteRec *consensusproto.RawRecord
	InviteKey crypto.PrivKey
}

type BatchResult struct {
	Rec     *consensusproto.RawRecord
	Invites []crypto.PrivKey
}

type AclRecordBuilder interface {
	UnmarshallWithId(rawIdRecord *consensusproto.RawRecordWithId) (rec *AclRecord, err error)
	Unmarshall(rawRecord *consensusproto.RawRecord) (rec *AclRecord, err error)

	BuildRoot(content RootContent) (rec *consensusproto.RawRecordWithId, err error)
	BuildBatchRequest(payload BatchRequestPayload) (batchResult BatchResult, err error)
	BuildInvite() (res InviteResult, err error)
	BuildInviteAnyone(permissions AclPermissions) (res InviteResult, err error)
	BuildInviteChange(inviteChange InviteChangePayload) (rawRecord *consensusproto.RawRecord, err error)
	BuildInviteRevoke(inviteRecordId string) (rawRecord *consensusproto.RawRecord, err error)
	BuildInviteJoin(payload InviteJoinPayload) (rawRecord *consensusproto.RawRecord, err error)
	BuildRequestJoin(payload RequestJoinPayload) (rawRecord *consensusproto.RawRecord, err error)
	BuildRequestAccept(payload RequestAcceptPayload) (rawRecord *consensusproto.RawRecord, err error)
	BuildRequestDecline(requestRecordId string) (rawRecord *consensusproto.RawRecord, err error)
	BuildRequestCancel(requestRecordId string) (rawRecord *consensusproto.RawRecord, err error)
	BuildRequestRemove() (rawRecord *consensusproto.RawRecord, err error)
	BuildPermissionChange(payload PermissionChangePayload) (rawRecord *consensusproto.RawRecord, err error)
	BuildPermissionChanges(payload PermissionChangesPayload) (rawRecord *consensusproto.RawRecord, err error)
	BuildReadKeyChange(payload ReadKeyChangePayload) (rawRecord *consensusproto.RawRecord, err error)
	BuildAccountRemove(payload AccountRemovePayload) (rawRecord *consensusproto.RawRecord, err error)
	BuildAccountsAdd(payload AccountsAddPayload) (rawRecord *consensusproto.RawRecord, err error)
}

type aclRecordBuilder struct {
	id          string
	keyStorage  crypto.KeyStorage
	accountKeys *accountdata.AccountKeys
	verifier    recordverifier.AcceptorVerifier
	state       *AclState
}

func NewAclRecordBuilder(id string, keyStorage crypto.KeyStorage, keys *accountdata.AccountKeys, verifier recordverifier.AcceptorVerifier) AclRecordBuilder {
	return &aclRecordBuilder{
		id:          id,
		keyStorage:  keyStorage,
		accountKeys: keys,
		verifier:    verifier,
	}
}

func (a *aclRecordBuilder) BuildBatchRequest(payload BatchRequestPayload) (batchResult BatchResult, err error) {
	var (
		contentList []*aclrecordproto.AclContentValue
		content     *aclrecordproto.AclContentValue
	)
	if len(payload.Removals.Identities) > 0 {
		content, err = a.buildAccountRemove(payload.Removals)
		if err != nil {
			return
		}
		contentList = append(contentList, content)
	}
	if len(payload.Additions) > 0 {
		content, err = a.buildAccountsAdd(AccountsAddPayload{Additions: payload.Additions}, payload.Removals.Change.MetadataKey.GetPublic(), payload.Removals.Change.ReadKey)
		if err != nil {
			return
		}
		contentList = append(contentList, content)
	}
	if len(payload.Changes) > 0 {
		content, err = a.buildPermissionChanges(PermissionChangesPayload{Changes: payload.Changes})
		if err != nil {
			return
		}
		contentList = append(contentList, content)
	}
	for _, acc := range payload.Approvals {
		content, err = a.buildRequestAccept(acc, payload.Removals.Change.ReadKey)
		if err != nil {
			return
		}
		contentList = append(contentList, content)
	}
	for _, id := range payload.Declines {
		content, err = a.buildRequestDecline(id)
		if err != nil {
			return
		}
		contentList = append(contentList, content)
	}
	for _, id := range payload.InviteRevokes {
		content, err = a.buildInviteRevoke(id)
		if err != nil {
			return
		}
		contentList = append(contentList, content)
	}
	for _, invite := range payload.InviteChanges {
		content, err = a.buildInviteChange(invite)
		if err != nil {
			return
		}
		contentList = append(contentList, content)
	}
	for _, perms := range payload.NewInvites {
		var privKey crypto.PrivKey
		if perms.NoPermissions() {
			privKey, content, err = a.buildInvite()
			if err != nil {
				return
			}
			contentList = append(contentList, content)
			batchResult.Invites = append(batchResult.Invites, privKey)
		} else {
			privKey, content, err = a.buildInviteAnyone(perms)
			if err != nil {
				return
			}
			contentList = append(contentList, content)
			batchResult.Invites = append(batchResult.Invites, privKey)
		}
	}
	res, err := a.buildRecords(contentList)
	if err != nil {
		return
	}
	batchResult.Rec = res
	return
}

func (a *aclRecordBuilder) buildRecord(aclContent *aclrecordproto.AclContentValue) (rawRec *consensusproto.RawRecord, err error) {
	return a.buildRecords([]*aclrecordproto.AclContentValue{aclContent})
}

func (a *aclRecordBuilder) buildRecords(aclContent []*aclrecordproto.AclContentValue) (rawRec *consensusproto.RawRecord, err error) {
	aclData := &aclrecordproto.AclData{AclContent: aclContent}
	marshalledData, err := aclData.Marshal()
	if err != nil {
		return
	}
	protoKey, err := a.accountKeys.SignKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	rec := &consensusproto.Record{
		PrevId:    a.state.lastRecordId,
		Identity:  protoKey,
		Data:      marshalledData,
		Timestamp: time.Now().Unix(),
	}
	marshalledRec, err := rec.Marshal()
	if err != nil {
		return
	}
	signature, err := a.accountKeys.SignKey.Sign(marshalledRec)
	if err != nil {
		return
	}
	rawRec = &consensusproto.RawRecord{
		Payload:   marshalledRec,
		Signature: signature,
	}
	err = a.preflightCheck(rawRec)
	return
}

func (a *aclRecordBuilder) preflightCheck(rawRecord *consensusproto.RawRecord) (err error) {
	aclRec, err := a.Unmarshall(rawRecord)
	if err != nil {
		return
	}
	cp := a.state.Copy()
	cp.contentValidator.(*contentValidator).verifier = recordverifier.NewValidateFull()
	return cp.ApplyRecord(aclRec)
}

func (a *aclRecordBuilder) BuildPermissionChanges(payload PermissionChangesPayload) (rawRecord *consensusproto.RawRecord, err error) {
	content, err := a.buildPermissionChanges(payload)
	if err != nil {
		return
	}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) buildPermissionChanges(payload PermissionChangesPayload) (content *aclrecordproto.AclContentValue, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	var changes []*aclrecordproto.AclAccountPermissionChange
	for _, perm := range payload.Changes {
		if perm.Identity.Equals(a.state.pubKey) {
			err = ErrInsufficientPermissions
			return
		}
		if perm.Permissions.IsOwner() {
			err = ErrIsOwner
			return
		}
		protoIdentity, err := perm.Identity.Marshall()
		if err != nil {
			return nil, err
		}
		changes = append(changes, &aclrecordproto.AclAccountPermissionChange{
			Identity:    protoIdentity,
			Permissions: aclrecordproto.AclUserPermissions(perm.Permissions),
		})
	}
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_PermissionChanges{
		&aclrecordproto.AclAccountPermissionChanges{changes},
	}}, nil
}

func (a *aclRecordBuilder) BuildAccountsAdd(payload AccountsAddPayload) (rawRecord *consensusproto.RawRecord, err error) {
	content, err := a.buildAccountsAdd(payload, nil, nil)
	if err != nil {
		return
	}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) buildAccountsAdd(payload AccountsAddPayload, mkKey crypto.PubKey, readKey crypto.SymKey) (value *aclrecordproto.AclContentValue, err error) {
	var accs []*aclrecordproto.AclAccountAdd
	for _, acc := range payload.Additions {
		if !a.state.Permissions(acc.Identity).NoPermissions() {
			return nil, ErrDuplicateAccounts
		}
		if acc.Permissions.IsOwner() {
			return nil, ErrIsOwner
		}
		if mkKey == nil {
			mkKey, err = a.state.CurrentMetadataKey()
			if err != nil {
				return nil, err
			}
		}
		encMeta, err := mkKey.Encrypt(acc.Metadata)
		if err != nil {
			return nil, err
		}
		if len(encMeta) > MaxMetadataLen {
			return nil, ErrMetadataTooLarge
		}
		if readKey == nil {
			readKey, err = a.state.CurrentReadKey()
			if err != nil {
				return nil, ErrNoReadKey
			}
		}
		protoKey, err := readKey.Marshall()
		if err != nil {
			return nil, err
		}
		enc, err := acc.Identity.Encrypt(protoKey)
		if err != nil {
			return nil, err
		}
		protoIdentity, err := acc.Identity.Marshall()
		if err != nil {
			return nil, err
		}
		accs = append(accs, &aclrecordproto.AclAccountAdd{
			Identity:         protoIdentity,
			Permissions:      aclrecordproto.AclUserPermissions(acc.Permissions),
			Metadata:         encMeta,
			EncryptedReadKey: enc,
		})
	}
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_AccountsAdd{
		&aclrecordproto.AclAccountsAdd{accs},
	}}, nil
}

func (a *aclRecordBuilder) BuildInvite() (res InviteResult, err error) {
	privKey, content, err := a.buildInvite()
	if err != nil {
		return
	}
	rawRec, err := a.buildRecord(content)
	if err != nil {
		return
	}
	res.InviteKey = privKey
	res.InviteRec = rawRec
	return
}

func (a *aclRecordBuilder) buildInvite() (invKey crypto.PrivKey, content *aclrecordproto.AclContentValue, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	privKey, pubKey, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return
	}
	invitePubKey, err := pubKey.Marshall()
	if err != nil {
		return
	}
	inviteRec := &aclrecordproto.AclAccountInvite{InviteKey: invitePubKey}
	content = &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_Invite{Invite: inviteRec}}
	invKey = privKey
	return
}

func (a *aclRecordBuilder) BuildInviteChange(inviteChange InviteChangePayload) (rawRecord *consensusproto.RawRecord, err error) {
	content, err := a.buildInviteChange(inviteChange)
	if err != nil {
		return
	}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) buildInviteChange(inviteChange InviteChangePayload) (content *aclrecordproto.AclContentValue, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	inviteRec := &aclrecordproto.AclAccountInviteChange{
		InviteRecordId: inviteChange.IniviteRecordId,
		Permissions:    aclrecordproto.AclUserPermissions(inviteChange.Permissions),
	}
	content = &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_InviteChange{InviteChange: inviteRec}}
	return
}

func (a *aclRecordBuilder) BuildInviteAnyone(permissions AclPermissions) (res InviteResult, err error) {
	privKey, content, err := a.buildInviteAnyone(permissions)
	if err != nil {
		return
	}
	rawRec, err := a.buildRecord(content)
	if err != nil {
		return
	}
	res.InviteKey = privKey
	res.InviteRec = rawRec
	return
}

func (a *aclRecordBuilder) buildInviteAnyone(permissions AclPermissions) (invKey crypto.PrivKey, content *aclrecordproto.AclContentValue, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	privKey, pubKey, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return
	}
	invitePubKey, err := pubKey.Marshall()
	if err != nil {
		return
	}
	curReadKey, err := a.state.CurrentReadKey()
	if err != nil {
		return
	}
	raw, err := curReadKey.Marshall()
	if err != nil {
		return
	}
	encReadKey, err := pubKey.Encrypt(raw)
	if err != nil {
		return
	}
	inviteRec := &aclrecordproto.AclAccountInvite{
		InviteKey:        invitePubKey,
		InviteType:       aclrecordproto.AclInviteType_AnyoneCanJoin,
		Permissions:      aclrecordproto.AclUserPermissions(permissions),
		EncryptedReadKey: encReadKey,
	}
	content = &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_Invite{Invite: inviteRec}}
	invKey = privKey
	return
}

func (a *aclRecordBuilder) BuildInviteRevoke(inviteRecordId string) (rawRecord *consensusproto.RawRecord, err error) {
	content, err := a.buildInviteRevoke(inviteRecordId)
	if err != nil {
		return
	}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) buildInviteRevoke(inviteRecordId string) (value *aclrecordproto.AclContentValue, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	_, exists := a.state.invites[inviteRecordId]
	if !exists {
		err = ErrNoSuchInvite
		return
	}
	revokeRec := &aclrecordproto.AclAccountInviteRevoke{InviteRecordId: inviteRecordId}
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_InviteRevoke{InviteRevoke: revokeRec}}, nil
}

func (a *aclRecordBuilder) BuildRequestJoin(payload RequestJoinPayload) (rawRecord *consensusproto.RawRecord, err error) {
	var inviteId string
	for id, inv := range a.state.invites {
		if inv.Key.Equals(payload.InviteKey.GetPublic()) && inv.Type == aclrecordproto.AclInviteType_RequestToJoin {
			inviteId = id
		}
	}
	invite, exists := a.state.invites[inviteId]
	if !exists {
		err = ErrNoSuchInvite
		return
	}
	if !payload.InviteKey.GetPublic().Equals(invite.Key) {
		err = ErrIncorrectInviteKey
		return
	}
	if !a.state.Permissions(a.accountKeys.SignKey.GetPublic()).NoPermissions() {
		err = ErrInsufficientPermissions
		return
	}
	mkKey, err := a.state.CurrentMetadataKey()
	if err != nil {
		return nil, err
	}
	encMeta, err := mkKey.Encrypt(payload.Metadata)
	if err != nil {
		return nil, err
	}
	if len(encMeta) > MaxMetadataLen {
		return nil, ErrMetadataTooLarge
	}
	rawIdentity, err := a.accountKeys.SignKey.GetPublic().Raw()
	if err != nil {
		return
	}
	signature, err := payload.InviteKey.Sign(rawIdentity)
	if err != nil {
		return
	}
	protoIdentity, err := a.accountKeys.SignKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	joinRec := &aclrecordproto.AclAccountRequestJoin{
		InviteIdentity:          protoIdentity,
		InviteRecordId:          inviteId,
		InviteIdentitySignature: signature,
		Metadata:                encMeta,
	}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestJoin{RequestJoin: joinRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) BuildInviteJoin(payload InviteJoinPayload) (rawRecord *consensusproto.RawRecord, err error) {
	var inviteId string
	for id, inv := range a.state.invites {
		if inv.Key.Equals(payload.InviteKey.GetPublic()) && inv.Type == aclrecordproto.AclInviteType_AnyoneCanJoin {
			inviteId = id
		}
	}
	invite, exists := a.state.invites[inviteId]
	if !exists {
		err = ErrNoSuchInvite
		return
	}
	if !payload.InviteKey.GetPublic().Equals(invite.Key) {
		err = ErrIncorrectInviteKey
		return
	}
	if !a.state.Permissions(a.accountKeys.SignKey.GetPublic()).NoPermissions() {
		err = ErrInsufficientPermissions
		return
	}
	mkKey, err := a.state.CurrentMetadataKey()
	if err != nil {
		return nil, err
	}
	encMeta, err := mkKey.Encrypt(payload.Metadata)
	if err != nil {
		return nil, err
	}
	if len(encMeta) > MaxMetadataLen {
		return nil, ErrMetadataTooLarge
	}
	rawIdentity, err := a.accountKeys.SignKey.GetPublic().Raw()
	if err != nil {
		return
	}
	signature, err := payload.InviteKey.Sign(rawIdentity)
	if err != nil {
		return
	}
	key, err := a.state.DecryptInvite(payload.InviteKey)
	if err != nil {
		return
	}
	readKey, err := key.Marshall()
	if err != nil {
		return
	}
	encReadKey, err := a.accountKeys.SignKey.GetPublic().Encrypt(readKey)
	if err != nil {
		return
	}
	protoIdentity, err := a.accountKeys.SignKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	joinRec := &aclrecordproto.AclAccountInviteJoin{
		Identity:                protoIdentity,
		InviteRecordId:          inviteId,
		InviteIdentitySignature: signature,
		Metadata:                encMeta,
		EncryptedReadKey:        encReadKey,
	}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_InviteJoin{InviteJoin: joinRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) BuildRequestAccept(payload RequestAcceptPayload) (rawRecord *consensusproto.RawRecord, err error) {
	content, err := a.buildRequestAccept(payload, nil)
	if err != nil {
		return
	}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) buildRequestAccept(payload RequestAcceptPayload, readKey crypto.SymKey) (value *aclrecordproto.AclContentValue, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	request, exists := a.state.requestRecords[payload.RequestRecordId]
	if !exists {
		err = ErrNoSuchRequest
		return
	}
	if readKey == nil {
		readKey, err = a.state.CurrentReadKey()
		if err != nil {
			return nil, ErrNoReadKey
		}
	}
	protoKey, err := readKey.Marshall()
	if err != nil {
		return nil, err
	}
	enc, err := request.RequestIdentity.Encrypt(protoKey)
	if err != nil {
		return nil, err
	}
	requestIdentityProto, err := request.RequestIdentity.Marshall()
	if err != nil {
		return
	}
	acceptRec := &aclrecordproto.AclAccountRequestAccept{
		Identity:         requestIdentityProto,
		RequestRecordId:  payload.RequestRecordId,
		EncryptedReadKey: enc,
		Permissions:      aclrecordproto.AclUserPermissions(payload.Permissions),
	}
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestAccept{RequestAccept: acceptRec}}, nil
}

func (a *aclRecordBuilder) BuildRequestDecline(requestRecordId string) (rawRecord *consensusproto.RawRecord, err error) {
	content, err := a.buildRequestDecline(requestRecordId)
	if err != nil {
		return
	}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) buildRequestDecline(requestRecordId string) (value *aclrecordproto.AclContentValue, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	rec, exists := a.state.requestRecords[requestRecordId]
	if !exists || rec.Type != RequestTypeJoin {
		err = ErrNoSuchRequest
		return
	}
	declineRec := &aclrecordproto.AclAccountRequestDecline{RequestRecordId: requestRecordId}
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestDecline{RequestDecline: declineRec}}, nil
}

func (a *aclRecordBuilder) BuildRequestCancel(requestRecordId string) (rawRecord *consensusproto.RawRecord, err error) {
	rec, exists := a.state.requestRecords[requestRecordId]
	if !exists {
		err = ErrNoSuchRequest
		return
	}
	if !rec.RequestIdentity.Equals(a.state.pubKey) {
		err = ErrInsufficientPermissions
		return
	}
	cancelRec := &aclrecordproto.AclAccountRequestCancel{RecordId: requestRecordId}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestCancel{RequestCancel: cancelRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) BuildPermissionChange(payload PermissionChangePayload) (rawRecord *consensusproto.RawRecord, err error) {
	permissions := a.state.Permissions(a.state.pubKey)
	if !permissions.CanManageAccounts() || payload.Identity.Equals(a.state.pubKey) {
		err = ErrInsufficientPermissions
		return
	}
	if payload.Permissions.IsOwner() {
		err = ErrIsOwner
		return
	}
	protoIdentity, err := payload.Identity.Marshall()
	if err != nil {
		return
	}
	permissionRec := &aclrecordproto.AclAccountPermissionChange{
		Identity:    protoIdentity,
		Permissions: aclrecordproto.AclUserPermissions(payload.Permissions),
	}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_PermissionChange{PermissionChange: permissionRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) BuildReadKeyChange(payload ReadKeyChangePayload) (rawRecord *consensusproto.RawRecord, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	rkChange, err := a.buildReadKeyChange(payload, nil)
	if err != nil {
		return nil, err
	}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: rkChange}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) buildReadKeyChange(payload ReadKeyChangePayload, removedIdentities map[string]struct{}) (*aclrecordproto.AclReadKeyChange, error) {
	// encrypting new read key with all keys of users
	protoKey, err := payload.ReadKey.Marshall()
	if err != nil {
		return nil, err
	}
	var (
		aclReadKeys []*aclrecordproto.AclEncryptedReadKey
		invites     []*aclrecordproto.AclEncryptedReadKey
	)
	for identity, st := range a.state.accountStates {
		if removedIdentities != nil {
			if _, exists := removedIdentities[identity]; exists {
				continue
			}
		}
		if st.Permissions.NoPermissions() {
			continue
		}
		protoIdentity, err := st.PubKey.Marshall()
		if err != nil {
			return nil, err
		}
		enc, err := st.PubKey.Encrypt(protoKey)
		if err != nil {
			return nil, err
		}
		aclReadKeys = append(aclReadKeys, &aclrecordproto.AclEncryptedReadKey{
			Identity:         protoIdentity,
			EncryptedReadKey: enc,
		})
	}
	for _, invite := range a.state.invites {
		if invite.Type != aclrecordproto.AclInviteType_AnyoneCanJoin {
			continue
		}
		protoIdentity, err := invite.Key.Marshall()
		if err != nil {
			return nil, err
		}
		enc, err := invite.Key.Encrypt(protoKey)
		if err != nil {
			return nil, err
		}
		invites = append(invites, &aclrecordproto.AclEncryptedReadKey{
			Identity:         protoIdentity,
			EncryptedReadKey: enc,
		})
	}
	// encrypting metadata key with new read key
	mkPubKey, err := payload.MetadataKey.GetPublic().Marshall()
	if err != nil {
		return nil, err
	}
	mkPrivKeyProto, err := payload.MetadataKey.Marshall()
	if err != nil {
		return nil, err
	}
	encPrivKey, err := payload.ReadKey.Encrypt(mkPrivKeyProto)
	if err != nil {
		return nil, err
	}
	// encrypting current read key with new read key
	curKey, err := a.state.CurrentReadKey()
	if err != nil {
		return nil, err
	}
	curKeyProto, err := curKey.Marshall()
	if err != nil {
		return nil, err
	}
	encOldKey, err := payload.ReadKey.Encrypt(curKeyProto)
	if err != nil {
		return nil, err
	}
	readRec := &aclrecordproto.AclReadKeyChange{
		AccountKeys:              aclReadKeys,
		MetadataPubKey:           mkPubKey,
		EncryptedMetadataPrivKey: encPrivKey,
		EncryptedOldReadKey:      encOldKey,
		InviteKeys:               invites,
	}
	return readRec, nil
}

func (a *aclRecordBuilder) BuildAccountRemove(payload AccountRemovePayload) (rawRecord *consensusproto.RawRecord, err error) {
	content, err := a.buildAccountRemove(payload)
	if err != nil {
		return
	}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) buildAccountRemove(payload AccountRemovePayload) (value *aclrecordproto.AclContentValue, err error) {
	deletedMap := map[string]struct{}{}
	for _, key := range payload.Identities {
		permissions := a.state.Permissions(key)
		if permissions.IsOwner() {
			return nil, ErrInsufficientPermissions
		}
		if permissions.NoPermissions() {
			return nil, ErrNoSuchAccount
		}
		deletedMap[mapKeyFromPubKey(key)] = struct{}{}
	}
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	var marshalledIdentities [][]byte
	for _, key := range payload.Identities {
		protoIdentity, err := key.Marshall()
		if err != nil {
			return nil, err
		}
		marshalledIdentities = append(marshalledIdentities, protoIdentity)
	}
	rkChange, err := a.buildReadKeyChange(payload.Change, deletedMap)
	if err != nil {
		return nil, err
	}
	removeRec := &aclrecordproto.AclAccountRemove{ReadKeyChange: rkChange, Identities: marshalledIdentities}
	return &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_AccountRemove{AccountRemove: removeRec}}, nil
}

func (a *aclRecordBuilder) BuildRequestRemove() (rawRecord *consensusproto.RawRecord, err error) {
	permissions := a.state.Permissions(a.state.pubKey)
	if permissions.NoPermissions() {
		err = ErrNoSuchAccount
		return
	}
	if permissions.IsOwner() {
		err = ErrIsOwner
		return
	}
	_, err = a.state.Record(a.state.pubKey)
	if !errors.Is(err, ErrNoSuchRecord) {
		return nil, ErrPendingRequest
	}
	removeRec := &aclrecordproto.AclAccountRequestRemove{}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_AccountRequestRemove{AccountRequestRemove: removeRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) Unmarshall(rawRecord *consensusproto.RawRecord) (rec *AclRecord, err error) {
	aclRecord := &consensusproto.Record{}
	err = proto.Unmarshal(rawRecord.Payload, aclRecord)
	if err != nil {
		return
	}
	pubKey, err := a.keyStorage.PubKeyFromProto(aclRecord.Identity)
	if err != nil {
		return
	}
	aclData := &aclrecordproto.AclData{}
	err = proto.Unmarshal(aclRecord.Data, aclData)
	if err != nil {
		return
	}
	rec = &AclRecord{
		PrevId:            aclRecord.PrevId,
		Timestamp:         aclRecord.Timestamp,
		AcceptorTimestamp: rawRecord.AcceptorTimestamp,
		Data:              aclRecord.Data,
		Signature:         rawRecord.Signature,
		Identity:          pubKey,
		Model:             aclData,
	}
	res, err := pubKey.Verify(rawRecord.Payload, rawRecord.Signature)
	if err != nil {
		return
	}
	if !res {
		err = ErrInvalidSignature
		return
	}
	return
}

func (a *aclRecordBuilder) UnmarshallWithId(rawIdRecord *consensusproto.RawRecordWithId) (rec *AclRecord, err error) {
	var (
		rawRec = &consensusproto.RawRecord{}
		pubKey crypto.PubKey
	)
	err = proto.Unmarshal(rawIdRecord.Payload, rawRec)
	if err != nil {
		return
	}
	if rawIdRecord.Id == a.id {
		aclRoot := &aclrecordproto.AclRoot{}
		err = proto.Unmarshal(rawRec.Payload, aclRoot)
		if err != nil {
			return
		}
		pubKey, err = a.keyStorage.PubKeyFromProto(aclRoot.Identity)
		if err != nil {
			return
		}
		rec = &AclRecord{
			Id:        rawIdRecord.Id,
			Timestamp: aclRoot.Timestamp,
			Signature: rawRec.Signature,
			Identity:  pubKey,
			Model:     aclRoot,
		}
	} else {
		err = a.verifier.VerifyAcceptor(rawRec)
		if err != nil {
			return
		}
		aclRecord := &consensusproto.Record{}
		err = proto.Unmarshal(rawRec.Payload, aclRecord)
		if err != nil {
			return
		}
		pubKey, err = a.keyStorage.PubKeyFromProto(aclRecord.Identity)
		if err != nil {
			return
		}
		aclData := &aclrecordproto.AclData{}
		err = proto.Unmarshal(aclRecord.Data, aclData)
		if err != nil {
			return
		}
		rec = &AclRecord{
			Id:        rawIdRecord.Id,
			PrevId:    aclRecord.PrevId,
			Timestamp: aclRecord.Timestamp,
			Data:      aclRecord.Data,
			Signature: rawRec.Signature,
			Identity:  pubKey,
			Model:     aclData,
		}
	}

	err = verifyRaw(pubKey, rawRec, rawIdRecord)
	return
}

func (a *aclRecordBuilder) BuildRoot(content RootContent) (rec *consensusproto.RawRecordWithId, err error) {
	rawIdentity, err := content.PrivKey.GetPublic().Raw()
	if err != nil {
		return
	}
	identity, err := content.PrivKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	masterKey, err := content.MasterKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	identitySignature, err := content.MasterKey.Sign(rawIdentity)
	if err != nil {
		return
	}
	aclRoot := &aclrecordproto.AclRoot{
		Identity:          identity,
		SpaceId:           content.SpaceId,
		MasterKey:         masterKey,
		IdentitySignature: identitySignature,
	}
	if content.Change.ReadKey != nil {
		aclRoot.Timestamp = time.Now().Unix()
		metadataPrivProto, err := content.Change.MetadataKey.Marshall()
		if err != nil {
			return nil, err
		}
		aclRoot.EncryptedMetadataPrivKey, err = content.Change.ReadKey.Encrypt(metadataPrivProto)
		if err != nil {
			return nil, err
		}
		aclRoot.MetadataPubKey, err = content.Change.MetadataKey.GetPublic().Marshall()
		if err != nil {
			return nil, err
		}
		rkProto, err := content.Change.ReadKey.Marshall()
		if err != nil {
			return nil, err
		}
		aclRoot.EncryptedReadKey, err = content.PrivKey.GetPublic().Encrypt(rkProto)
		if err != nil {
			return nil, err
		}
		enc, err := content.Change.MetadataKey.GetPublic().Encrypt(content.Metadata)
		if err != nil {
			return nil, err
		}
		aclRoot.EncryptedOwnerMetadata = enc
	}
	return marshalAclRoot(aclRoot, content.PrivKey)
}

func verifyRaw(
	pubKey crypto.PubKey,
	rawRec *consensusproto.RawRecord,
	recWithId *consensusproto.RawRecordWithId) (err error) {
	// verifying signature
	res, err := pubKey.Verify(rawRec.Payload, rawRec.Signature)
	if err != nil {
		return
	}
	if !res {
		err = ErrInvalidSignature
		return
	}

	// verifying ID
	if !cidutil.VerifyCid(recWithId.Payload, recWithId.Id) {
		err = ErrIncorrectCID
	}
	return
}

func marshalAclRoot(aclRoot *aclrecordproto.AclRoot, key crypto.PrivKey) (rawWithId *consensusproto.RawRecordWithId, err error) {
	marshalledRoot, err := aclRoot.Marshal()
	if err != nil {
		return
	}
	signature, err := key.Sign(marshalledRoot)
	if err != nil {
		return
	}
	raw := &consensusproto.RawRecord{
		Payload:   marshalledRoot,
		Signature: signature,
	}
	marshalledRaw, err := raw.Marshal()
	if err != nil {
		return
	}
	aclHeadId, err := cidutil.NewCidFromBytes(marshalledRaw)
	if err != nil {
		return
	}
	rawWithId = &consensusproto.RawRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	return
}
