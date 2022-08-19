package acllistbuilder

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
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
	records    []*aclpb.Record
	rawRecords []*aclpb.RawRecord
	indexes    map[string]int
	keychain   *Keychain
	header     *aclpb.Header
	id         string
}

func NewACLListStorageBuilder(keychain *Keychain) *ACLListStorageBuilder {
	return &ACLListStorageBuilder{
		records:  make([]*aclpb.Record, 0),
		indexes:  make(map[string]int),
		keychain: keychain,
	}
}

func NewACLListStorageBuilderWithTestName(name string) (*ACLListStorageBuilder, error) {
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

func (t *ACLListStorageBuilder) createRaw(rec *aclpb.Record) *aclpb.RawRecord {
	aclMarshaled, err := proto.Marshal(rec)
	if err != nil {
		panic("should be able to marshal final acl message!")
	}

	signature, err := t.keychain.SigningKeysByIdentity[rec.Identity].Sign(aclMarshaled)
	if err != nil {
		panic("should be able to sign final acl message!")
	}

	id, _ := cid.NewCIDFromBytes(aclMarshaled)

	return &aclpb.RawRecord{
		Payload:   aclMarshaled,
		Signature: signature,
		Id:        id,
	}
}

func (t *ACLListStorageBuilder) getRecord(idx int) *aclpb.RawRecord {
	return t.rawRecords[idx]
}

func (t *ACLListStorageBuilder) Head() (*aclpb.RawRecord, error) {
	return t.getRecord(len(t.records) - 1), nil
}

func (t *ACLListStorageBuilder) Header() (*aclpb.Header, error) {
	return t.header, nil
}

func (t *ACLListStorageBuilder) GetRecord(ctx context.Context, id string) (*aclpb.RawRecord, error) {
	recIdx, ok := t.indexes[id]
	if !ok {
		return nil, fmt.Errorf("no such record")
	}
	return t.getRecord(recIdx), nil
}

func (t *ACLListStorageBuilder) AddRecord(ctx context.Context, rec *aclpb.Record) error {
	panic("implement me")
}

func (t *ACLListStorageBuilder) ID() (string, error) {
	return t.id, nil
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

func (t *ACLListStorageBuilder) parseRecord(rec *Record, prevId string) *aclpb.Record {
	k := t.keychain.GetKey(rec.ReadKey).(*SymKey)
	var aclChangeContents []*aclpb.ACLChangeACLContentValue
	for _, ch := range rec.AclChanges {
		aclChangeContent := t.parseACLChange(ch)
		aclChangeContents = append(aclChangeContents, aclChangeContent)
	}
	data := &aclpb.ACLChangeACLData{
		AclContent: aclChangeContents,
	}
	bytes, _ := data.Marshal()

	return &aclpb.Record{
		PrevId:             prevId,
		Identity:           t.keychain.GetIdentity(rec.Identity),
		Data:               bytes,
		CurrentReadKeyHash: k.Hash,
		Timestamp:          time.Now().Unix(),
	}
}

func (t *ACLListStorageBuilder) parseACLChange(ch *ACLChange) (convCh *aclpb.ACLChangeACLContentValue) {
	switch {
	case ch.UserAdd != nil:
		add := ch.UserAdd

		encKey := t.keychain.
			GetKey(add.EncryptionKey).(encryptionkey.PrivKey)
		rawKey, _ := encKey.GetPublic().Raw()

		convCh = &aclpb.ACLChangeACLContentValue{
			Value: &aclpb.ACLChangeACLContentValueValueOfUserAdd{
				UserAdd: &aclpb.ACLChangeUserAdd{
					Identity:          t.keychain.GetIdentity(add.Identity),
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

		convCh = &aclpb.ACLChangeACLContentValue{
			Value: &aclpb.ACLChangeACLContentValueValueOfUserJoin{
				UserJoin: &aclpb.ACLChangeUserJoin{
					Identity:          t.keychain.GetIdentity(join.Identity),
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

		convCh = &aclpb.ACLChangeACLContentValue{
			Value: &aclpb.ACLChangeACLContentValueValueOfUserInvite{
				UserInvite: &aclpb.ACLChangeUserInvite{
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

		convCh = &aclpb.ACLChangeACLContentValue{
			Value: &aclpb.ACLChangeACLContentValueValueOfUserConfirm{
				UserConfirm: &aclpb.ACLChangeUserConfirm{
					Identity:  t.keychain.GetIdentity(confirm.Identity),
					UserAddId: confirm.UserAddId,
				},
			},
		}
	case ch.UserPermissionChange != nil:
		permissionChange := ch.UserPermissionChange

		convCh = &aclpb.ACLChangeACLContentValue{
			Value: &aclpb.ACLChangeACLContentValueValueOfUserPermissionChange{
				UserPermissionChange: &aclpb.ACLChangeUserPermissionChange{
					Identity:    t.keychain.GetIdentity(permissionChange.Identity),
					Permissions: t.convertPermission(permissionChange.Permission),
				},
			},
		}
	case ch.UserRemove != nil:
		remove := ch.UserRemove

		newReadKey := t.keychain.GetKey(remove.NewReadKey).(*SymKey)

		var replaces []*aclpb.ACLChangeReadKeyReplace
		for _, id := range remove.IdentitiesLeft {
			identity := t.keychain.GetIdentity(id)
			encKey := t.keychain.EncryptionKeys[id]
			rawEncKey, _ := encKey.GetPublic().Raw()
			encReadKey, err := encKey.GetPublic().Encrypt(newReadKey.Key.Bytes())
			if err != nil {
				panic(err)
			}
			replaces = append(replaces, &aclpb.ACLChangeReadKeyReplace{
				Identity:         identity,
				EncryptionKey:    rawEncKey,
				EncryptedReadKey: encReadKey,
			})
		}

		convCh = &aclpb.ACLChangeACLContentValue{
			Value: &aclpb.ACLChangeACLContentValueValueOfUserRemove{
				UserRemove: &aclpb.ACLChangeUserRemove{
					Identity:        t.keychain.GetIdentity(remove.RemovedIdentity),
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

func (t *ACLListStorageBuilder) convertPermission(perm string) aclpb.ACLChangeUserPermissions {
	switch perm {
	case "admin":
		return aclpb.ACLChange_Admin
	case "writer":
		return aclpb.ACLChange_Writer
	case "reader":
		return aclpb.ACLChange_Reader
	default:
		panic(fmt.Sprintf("incorrect permission: %s", perm))
	}
}

func (t *ACLListStorageBuilder) traverseFromHead(f func(rec *aclpb.Record, id string) error) (err error) {
	for i := len(t.records) - 1; i >= 0; i-- {
		err = f(t.records[i], t.rawRecords[i].Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *ACLListStorageBuilder) createHeaderAndId() {
	t.header = &aclpb.Header{
		FirstId:     t.rawRecords[0].Id,
		AclListId:   "",
		WorkspaceId: "",
		DocType:     aclpb.Header_ACL,
	}
	bytes, _ := t.header.Marshal()
	id, _ := cid.NewCIDFromBytes(bytes)
	t.id = id
}
