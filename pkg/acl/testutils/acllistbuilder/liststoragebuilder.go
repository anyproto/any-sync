package acllistbuilder

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/yamltests"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"io/ioutil"
	"path"
	"time"

	"github.com/gogo/protobuf/proto"
	"gopkg.in/yaml.v3"
)

type ACLListStorageBuilder struct {
	aclList    string
	records    []*aclpb.ACLRecord
	rawRecords []*aclpb.RawACLRecord
	indexes    map[string]int
	keychain   *Keychain
	header     *aclpb.ACLHeader
	id         string
}

func NewACLListStorageBuilder(keychain *Keychain) *ACLListStorageBuilder {
	return &ACLListStorageBuilder{
		records:  make([]*aclpb.ACLRecord, 0),
		indexes:  make(map[string]int),
		keychain: keychain,
	}
}

func NewListStorageWithTestName(name string) (storage.ListStorage, error) {
	filePath := path.Join(yamltests.Path(), name)
	return NewACLListStorageBuilderFromFile(filePath)
}

func NewACLListStorageBuilderFromFile(file string) (*ACLListStorageBuilder, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	ymlTree := YMLList{}
	err = yaml.Unmarshal(content, &ymlTree)
	if err != nil {
		return nil, err
	}

	tb := NewACLListStorageBuilder(NewKeychain())
	tb.Parse(&ymlTree)

	return tb, nil
}

func (t *ACLListStorageBuilder) createRaw(rec *aclpb.ACLRecord) *aclpb.RawACLRecord {
	aclMarshaled, err := proto.Marshal(rec)
	if err != nil {
		panic("should be able to marshal final acl message!")
	}

	signature, err := t.keychain.SigningKeysByIdentity[string(rec.Identity)].Sign(aclMarshaled)
	if err != nil {
		panic("should be able to sign final acl message!")
	}

	id, _ := cid.NewCIDFromBytes(aclMarshaled)

	return &aclpb.RawACLRecord{
		Payload:   aclMarshaled,
		Signature: signature,
		Id:        id,
	}
}

func (t *ACLListStorageBuilder) getRecord(idx int) *aclpb.RawACLRecord {
	return t.rawRecords[idx]
}

func (t *ACLListStorageBuilder) Head() (*aclpb.RawACLRecord, error) {
	return t.getRecord(len(t.records) - 1), nil
}

func (t *ACLListStorageBuilder) Header() (*aclpb.ACLHeader, error) {
	return t.header, nil
}

func (t *ACLListStorageBuilder) GetRawRecord(ctx context.Context, id string) (*aclpb.RawACLRecord, error) {
	recIdx, ok := t.indexes[id]
	if !ok {
		return nil, fmt.Errorf("no such record")
	}
	return t.getRecord(recIdx), nil
}

func (t *ACLListStorageBuilder) AddRawRecord(ctx context.Context, rec *aclpb.RawACLRecord) error {
	panic("implement me")
}

func (t *ACLListStorageBuilder) ID() (string, error) {
	return t.id, nil
}

func (t *ACLListStorageBuilder) GetRawRecords() []*aclpb.RawACLRecord {
	return t.rawRecords
}

func (t *ACLListStorageBuilder) GetKeychain() *Keychain {
	return t.keychain
}

func (t *ACLListStorageBuilder) Parse(tree *YMLList) {
	// Just to clarify - we are generating new identities for the ones that
	// are specified in the yml file, because our identities should be Ed25519
	// the same thing is happening for the encryption keys
	t.keychain.ParseKeys(&tree.Keys)
	prevId := ""
	for idx, rec := range tree.Records {
		newRecord := t.parseRecord(rec, prevId)
		rawRecord := t.createRaw(newRecord)
		t.records = append(t.records, newRecord)
		t.rawRecords = append(t.rawRecords, t.createRaw(newRecord))
		t.indexes[rawRecord.Id] = idx
		prevId = rawRecord.Id
	}

	t.createHeaderAndId()
}

func (t *ACLListStorageBuilder) parseRecord(rec *Record, prevId string) *aclpb.ACLRecord {
	k := t.keychain.GetKey(rec.ReadKey).(*SymKey)
	var aclChangeContents []*aclpb.ACLContentValue
	for _, ch := range rec.AclChanges {
		aclChangeContent := t.parseACLChange(ch)
		aclChangeContents = append(aclChangeContents, aclChangeContent)
	}
	data := &aclpb.ACLData{
		AclContent: aclChangeContents,
	}
	bytes, _ := data.Marshal()

	return &aclpb.ACLRecord{
		PrevId:             prevId,
		Identity:           []byte(t.keychain.GetIdentity(rec.Identity)),
		Data:               bytes,
		CurrentReadKeyHash: k.Hash,
		Timestamp:          time.Now().Unix(),
	}
}

func (t *ACLListStorageBuilder) parseACLChange(ch *ACLChange) (convCh *aclpb.ACLContentValue) {
	switch {
	case ch.UserAdd != nil:
		add := ch.UserAdd

		encKey := t.keychain.GetKey(add.EncryptionKey).(encryptionkey.PrivKey)
		rawKey, _ := encKey.GetPublic().Raw()

		convCh = &aclpb.ACLContentValue{
			Value: &aclpb.ACLContentValue_UserAdd{
				UserAdd: &aclpb.ACLUserAdd{
					Identity:          []byte(t.keychain.GetIdentity(add.Identity)),
					EncryptionKey:     rawKey,
					EncryptedReadKeys: t.encryptReadKeys(add.EncryptedReadKeys, encKey),
					Permissions:       t.convertPermission(add.Permission),
				},
			},
		}
	case ch.UserJoin != nil:
		join := ch.UserJoin

		encKey := t.keychain.
			GetKey(join.EncryptionKey).(encryptionkey.PrivKey)
		rawKey, _ := encKey.GetPublic().Raw()

		idKey, _ := t.keychain.SigningKeys[join.Identity].GetPublic().Raw()
		signKey := t.keychain.GetKey(join.AcceptSignature).(signingkey.PrivKey)
		signature, err := signKey.Sign(idKey)
		if err != nil {
			panic(err)
		}

		convCh = &aclpb.ACLContentValue{
			Value: &aclpb.ACLContentValue_UserJoin{
				UserJoin: &aclpb.ACLUserJoin{
					Identity:          []byte(t.keychain.GetIdentity(join.Identity)),
					EncryptionKey:     rawKey,
					AcceptSignature:   signature,
					UserInviteId:      join.InviteId,
					EncryptedReadKeys: t.encryptReadKeys(join.EncryptedReadKeys, encKey),
				},
			},
		}
	case ch.UserInvite != nil:
		invite := ch.UserInvite
		rawAcceptKey, _ := t.keychain.GetKey(invite.AcceptKey).(signingkey.PrivKey).GetPublic().Raw()
		encKey := t.keychain.
			GetKey(invite.EncryptionKey).(encryptionkey.PrivKey)
		rawEncKey, _ := encKey.GetPublic().Raw()

		convCh = &aclpb.ACLContentValue{
			Value: &aclpb.ACLContentValue_UserInvite{
				UserInvite: &aclpb.ACLUserInvite{
					AcceptPublicKey:   rawAcceptKey,
					EncryptPublicKey:  rawEncKey,
					EncryptedReadKeys: t.encryptReadKeys(invite.EncryptedReadKeys, encKey),
					Permissions:       t.convertPermission(invite.Permissions),
					InviteId:          invite.InviteId,
				},
			},
		}
	case ch.UserConfirm != nil:
		confirm := ch.UserConfirm

		convCh = &aclpb.ACLContentValue{
			Value: &aclpb.ACLContentValue_UserConfirm{
				UserConfirm: &aclpb.ACLUserConfirm{
					Identity:  []byte(t.keychain.GetIdentity(confirm.Identity)),
					UserAddId: confirm.UserAddId,
				},
			},
		}
	case ch.UserPermissionChange != nil:
		permissionChange := ch.UserPermissionChange

		convCh = &aclpb.ACLContentValue{
			Value: &aclpb.ACLContentValue_UserPermissionChange{
				UserPermissionChange: &aclpb.ACLUserPermissionChange{
					Identity:    []byte(t.keychain.GetIdentity(permissionChange.Identity)),
					Permissions: t.convertPermission(permissionChange.Permission),
				},
			},
		}
	case ch.UserRemove != nil:
		remove := ch.UserRemove

		newReadKey := t.keychain.GetKey(remove.NewReadKey).(*SymKey)

		var replaces []*aclpb.ACLReadKeyReplace
		for _, id := range remove.IdentitiesLeft {
			identity := t.keychain.GetIdentity(id)
			encKey := t.keychain.EncryptionKeys[id]
			rawEncKey, _ := encKey.GetPublic().Raw()
			encReadKey, err := encKey.GetPublic().Encrypt(newReadKey.Key.Bytes())
			if err != nil {
				panic(err)
			}
			replaces = append(replaces, &aclpb.ACLReadKeyReplace{
				Identity:         []byte(identity),
				EncryptionKey:    rawEncKey,
				EncryptedReadKey: encReadKey,
			})
		}

		convCh = &aclpb.ACLContentValue{
			Value: &aclpb.ACLContentValue_UserRemove{
				UserRemove: &aclpb.ACLUserRemove{
					Identity:        []byte(t.keychain.GetIdentity(remove.RemovedIdentity)),
					ReadKeyReplaces: replaces,
				},
			},
		}
	}
	if convCh == nil {
		panic("cannot have empty acl change")
	}

	return convCh
}

func (t *ACLListStorageBuilder) encryptReadKeys(keys []string, encKey encryptionkey.PrivKey) (enc [][]byte) {
	for _, k := range keys {
		realKey := t.keychain.GetKey(k).(*SymKey).Key.Bytes()
		res, err := encKey.GetPublic().Encrypt(realKey)
		if err != nil {
			panic(err)
		}

		enc = append(enc, res)
	}
	return
}

func (t *ACLListStorageBuilder) convertPermission(perm string) aclpb.ACLUserPermissions {
	switch perm {
	case "admin":
		return aclpb.ACLUserPermissions_Admin
	case "writer":
		return aclpb.ACLUserPermissions_Writer
	case "reader":
		return aclpb.ACLUserPermissions_Reader
	default:
		panic(fmt.Sprintf("incorrect permission: %s", perm))
	}
}

func (t *ACLListStorageBuilder) traverseFromHead(f func(rec *aclpb.ACLRecord, id string) error) (err error) {
	for i := len(t.records) - 1; i >= 0; i-- {
		err = f(t.records[i], t.rawRecords[i].Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *ACLListStorageBuilder) createHeaderAndId() {
	t.header = &aclpb.ACLHeader{
		FirstId: t.rawRecords[0].Id,
	}
	bytes, _ := t.header.Marshal()
	id, _ := cid.NewCIDFromBytes(bytes)
	t.id = id
}
