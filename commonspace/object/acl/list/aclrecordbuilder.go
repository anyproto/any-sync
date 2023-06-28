package list

import (
	"time"

	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/gogo/protobuf/proto"
)

type RootContent struct {
	PrivKey          crypto.PrivKey
	MasterKey        crypto.PrivKey
	SpaceId          string
	EncryptedReadKey []byte
}

type RequestJoinPayload struct {
	InviteRecordId string
	InviteKey      crypto.PrivKey
	Metadata       []byte
}

type RequestAcceptPayload struct {
	RequestRecordId string
	Permissions     AclPermissions
}

type PermissionChangePayload struct {
	Identity    crypto.PubKey
	Permissions AclPermissions
}

type AccountRemovePayload struct {
	Identities []crypto.PubKey
	ReadKey    crypto.SymKey
}

type InviteResult struct {
	InviteRec *aclrecordproto.RawAclRecord
	InviteKey crypto.PrivKey
}

type AclRecordBuilder interface {
	UnmarshallWithId(rawIdRecord *aclrecordproto.RawAclRecordWithId) (rec *AclRecord, err error)
	Unmarshall(rawRecord *aclrecordproto.RawAclRecord) (rec *AclRecord, err error)

	BuildRoot(content RootContent) (rec *aclrecordproto.RawAclRecordWithId, err error)
	BuildInvite() (res InviteResult, err error)
	BuildInviteRevoke(inviteRecordId string) (rawRecord *aclrecordproto.RawAclRecord, err error)
	BuildRequestJoin(payload RequestJoinPayload) (rawRecord *aclrecordproto.RawAclRecord, err error)
	BuildRequestAccept(payload RequestAcceptPayload) (rawRecord *aclrecordproto.RawAclRecord, err error)
	BuildRequestDecline(requestRecordId string) (rawRecord *aclrecordproto.RawAclRecord, err error)
	BuildRequestRemove() (rawRecord *aclrecordproto.RawAclRecord, err error)
	BuildPermissionChange(payload PermissionChangePayload) (rawRecord *aclrecordproto.RawAclRecord, err error)
	BuildReadKeyChange(newKey crypto.SymKey) (rawRecord *aclrecordproto.RawAclRecord, err error)
	BuildAccountRemove(payload AccountRemovePayload) (rawRecord *aclrecordproto.RawAclRecord, err error)
}

type aclRecordBuilder struct {
	id          string
	keyStorage  crypto.KeyStorage
	accountKeys *accountdata.AccountKeys
	verifier    AcceptorVerifier
	state       *AclState
}

func NewAclRecordBuilder(id string, keyStorage crypto.KeyStorage, keys *accountdata.AccountKeys, verifier AcceptorVerifier) AclRecordBuilder {
	return &aclRecordBuilder{
		id:          id,
		keyStorage:  keyStorage,
		accountKeys: keys,
		verifier:    verifier,
	}
}

func (a *aclRecordBuilder) buildRecord(aclContent *aclrecordproto.AclContentValue) (rawRec *aclrecordproto.RawAclRecord, err error) {
	aclData := &aclrecordproto.AclData{AclContent: []*aclrecordproto.AclContentValue{
		aclContent,
	}}
	marshalledData, err := aclData.Marshal()
	if err != nil {
		return
	}
	protoKey, err := a.accountKeys.SignKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	rec := &aclrecordproto.AclRecord{
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
	rawRec = &aclrecordproto.RawAclRecord{
		Payload:   marshalledRec,
		Signature: signature,
	}
	return
}

func (a *aclRecordBuilder) BuildInvite() (res InviteResult, err error) {
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
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_Invite{Invite: inviteRec}}
	rawRec, err := a.buildRecord(content)
	if err != nil {
		return
	}
	res.InviteKey = privKey
	res.InviteRec = rawRec
	return
}

func (a *aclRecordBuilder) BuildInviteRevoke(inviteRecordId string) (rawRecord *aclrecordproto.RawAclRecord, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	_, exists := a.state.inviteKeys[inviteRecordId]
	if !exists {
		err = ErrNoSuchInvite
		return
	}
	revokeRec := &aclrecordproto.AclAccountInviteRevoke{InviteRecordId: inviteRecordId}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_InviteRevoke{InviteRevoke: revokeRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) BuildRequestJoin(payload RequestJoinPayload) (rawRecord *aclrecordproto.RawAclRecord, err error) {
	key, exists := a.state.inviteKeys[payload.InviteRecordId]
	if !exists {
		err = ErrNoSuchInvite
		return
	}
	if !payload.InviteKey.GetPublic().Equals(key) {
		err = ErrIncorrectInviteKey
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
		InviteRecordId:          payload.InviteRecordId,
		InviteIdentitySignature: signature,
		Metadata:                payload.Metadata,
	}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestJoin{RequestJoin: joinRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) BuildRequestAccept(payload RequestAcceptPayload) (rawRecord *aclrecordproto.RawAclRecord, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	request, exists := a.state.requestRecords[payload.RequestRecordId]
	if !exists {
		err = ErrNoSuchRequest
		return
	}
	var encryptedReadKeys []*aclrecordproto.AclReadKeyWithRecord
	for keyId, key := range a.state.userReadKeys {
		rawKey, err := key.Raw()
		if err != nil {
			return nil, err
		}
		enc, err := request.RequestIdentity.Encrypt(rawKey)
		if err != nil {
			return nil, err
		}
		encryptedReadKeys = append(encryptedReadKeys, &aclrecordproto.AclReadKeyWithRecord{
			RecordId:         keyId,
			EncryptedReadKey: enc,
		})
	}
	if err != nil {
		return
	}
	requestIdentityProto, err := request.RequestIdentity.Marshall()
	if err != nil {
		return
	}
	acceptRec := &aclrecordproto.AclAccountRequestAccept{
		Identity:          requestIdentityProto,
		RequestRecordId:   payload.RequestRecordId,
		EncryptedReadKeys: encryptedReadKeys,
		Permissions:       aclrecordproto.AclUserPermissions(payload.Permissions),
	}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestAccept{RequestAccept: acceptRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) BuildRequestDecline(requestRecordId string) (rawRecord *aclrecordproto.RawAclRecord, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	_, exists := a.state.requestRecords[requestRecordId]
	if !exists {
		err = ErrNoSuchRequest
		return
	}
	declineRec := &aclrecordproto.AclAccountRequestDecline{RequestRecordId: requestRecordId}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_RequestDecline{RequestDecline: declineRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) BuildPermissionChange(payload PermissionChangePayload) (rawRecord *aclrecordproto.RawAclRecord, err error) {
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

func (a *aclRecordBuilder) BuildReadKeyChange(newKey crypto.SymKey) (rawRecord *aclrecordproto.RawAclRecord, err error) {
	if !a.state.Permissions(a.state.pubKey).CanManageAccounts() {
		err = ErrInsufficientPermissions
		return
	}
	rawKey, err := newKey.Raw()
	if err != nil {
		return
	}
	if len(rawKey) != crypto.KeyBytes {
		err = ErrIncorrectReadKey
		return
	}
	var aclReadKeys []*aclrecordproto.AclEncryptedReadKey
	for _, st := range a.state.userStates {
		protoIdentity, err := st.PubKey.Marshall()
		if err != nil {
			return nil, err
		}
		enc, err := st.PubKey.Encrypt(rawKey)
		if err != nil {
			return nil, err
		}
		aclReadKeys = append(aclReadKeys, &aclrecordproto.AclEncryptedReadKey{
			Identity:         protoIdentity,
			EncryptedReadKey: enc,
		})
	}
	readRec := &aclrecordproto.AclReadKeyChange{AccountKeys: aclReadKeys}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_ReadKeyChange{ReadKeyChange: readRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) BuildAccountRemove(payload AccountRemovePayload) (rawRecord *aclrecordproto.RawAclRecord, err error) {
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
	rawKey, err := payload.ReadKey.Raw()
	if err != nil {
		return
	}
	if len(rawKey) != crypto.KeyBytes {
		err = ErrIncorrectReadKey
		return
	}
	var aclReadKeys []*aclrecordproto.AclEncryptedReadKey
	for _, st := range a.state.userStates {
		if _, exists := deletedMap[mapKeyFromPubKey(st.PubKey)]; exists {
			continue
		}
		protoIdentity, err := st.PubKey.Marshall()
		if err != nil {
			return nil, err
		}
		enc, err := st.PubKey.Encrypt(rawKey)
		if err != nil {
			return nil, err
		}
		aclReadKeys = append(aclReadKeys, &aclrecordproto.AclEncryptedReadKey{
			Identity:         protoIdentity,
			EncryptedReadKey: enc,
		})
	}
	var marshalledIdentities [][]byte
	for _, key := range payload.Identities {
		protoIdentity, err := key.Marshall()
		if err != nil {
			return nil, err
		}
		marshalledIdentities = append(marshalledIdentities, protoIdentity)
	}
	removeRec := &aclrecordproto.AclAccountRemove{AccountKeys: aclReadKeys, Identities: marshalledIdentities}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_AccountRemove{AccountRemove: removeRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) BuildRequestRemove() (rawRecord *aclrecordproto.RawAclRecord, err error) {
	permissions := a.state.Permissions(a.state.pubKey)
	if permissions.NoPermissions() {
		err = ErrNoSuchAccount
		return
	}
	if permissions.IsOwner() {
		err = ErrIsOwner
		return
	}
	removeRec := &aclrecordproto.AclAccountRequestRemove{}
	content := &aclrecordproto.AclContentValue{Value: &aclrecordproto.AclContentValue_AccountRequestRemove{AccountRequestRemove: removeRec}}
	return a.buildRecord(content)
}

func (a *aclRecordBuilder) Unmarshall(rawRecord *aclrecordproto.RawAclRecord) (rec *AclRecord, err error) {
	aclRecord := &aclrecordproto.AclRecord{}
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
		PrevId:    aclRecord.PrevId,
		Timestamp: aclRecord.Timestamp,
		Data:      aclRecord.Data,
		Signature: rawRecord.Signature,
		Identity:  pubKey,
		Model:     aclData,
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

func (a *aclRecordBuilder) UnmarshallWithId(rawIdRecord *aclrecordproto.RawAclRecordWithId) (rec *AclRecord, err error) {
	var (
		rawRec = &aclrecordproto.RawAclRecord{}
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
		aclRecord := &aclrecordproto.AclRecord{}
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

func (a *aclRecordBuilder) BuildRoot(content RootContent) (rec *aclrecordproto.RawAclRecordWithId, err error) {
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
	var timestamp int64
	if content.EncryptedReadKey != nil {
		timestamp = time.Now().Unix()
	}
	aclRoot := &aclrecordproto.AclRoot{
		Identity:          identity,
		SpaceId:           content.SpaceId,
		EncryptedReadKey:  content.EncryptedReadKey,
		MasterKey:         masterKey,
		IdentitySignature: identitySignature,
		Timestamp:         timestamp,
	}
	return marshalAclRoot(aclRoot, content.PrivKey)
}

func verifyRaw(
	pubKey crypto.PubKey,
	rawRec *aclrecordproto.RawAclRecord,
	recWithId *aclrecordproto.RawAclRecordWithId) (err error) {
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

func marshalAclRoot(aclRoot *aclrecordproto.AclRoot, key crypto.PrivKey) (rawWithId *aclrecordproto.RawAclRecordWithId, err error) {
	marshalledRoot, err := aclRoot.Marshal()
	if err != nil {
		return
	}
	signature, err := key.Sign(marshalledRoot)
	if err != nil {
		return
	}
	raw := &aclrecordproto.RawAclRecord{
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
	rawWithId = &aclrecordproto.RawAclRecordWithId{
		Payload: marshalledRaw,
		Id:      aclHeadId,
	}
	return
}
