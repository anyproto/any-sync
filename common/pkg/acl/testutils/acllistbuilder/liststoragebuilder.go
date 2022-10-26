package acllistbuilder

import (
	"context"
	"fmt"
	aclrecordproto2 "github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/storage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/testutils/yamltests"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"hash/fnv"
	"io/ioutil"
	"path"
	"time"

	"github.com/gogo/protobuf/proto"
	"gopkg.in/yaml.v3"
)

type ACLListStorageBuilder struct {
	aclList    string
	records    []*aclrecordproto2.ACLRecord
	rawRecords []*aclrecordproto2.RawACLRecordWithId
	indexes    map[string]int
	keychain   *YAMLKeychain
	rawRoot    *aclrecordproto2.RawACLRecordWithId
	root       *aclrecordproto2.ACLRoot
	id         string
}

func NewACLListStorageBuilder(keychain *YAMLKeychain) *ACLListStorageBuilder {
	return &ACLListStorageBuilder{
		records:  make([]*aclrecordproto2.ACLRecord, 0),
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

func (t *ACLListStorageBuilder) createRaw(rec proto.Marshaler, identity []byte) *aclrecordproto2.RawACLRecordWithId {
	protoMarshalled, err := rec.Marshal()
	if err != nil {
		panic("should be able to marshal final acl message!")
	}

	signature, err := t.keychain.SigningKeysByRealIdentity[string(identity)].Sign(protoMarshalled)
	if err != nil {
		panic("should be able to sign final acl message!")
	}

	rawRec := &aclrecordproto2.RawACLRecord{
		Payload:   protoMarshalled,
		Signature: signature,
	}

	rawMarshalled, err := proto.Marshal(rawRec)
	if err != nil {
		panic(err)
	}

	id, _ := cid.NewCIDFromBytes(rawMarshalled)

	return &aclrecordproto2.RawACLRecordWithId{
		Payload: rawMarshalled,
		Id:      id,
	}
}

func (t *ACLListStorageBuilder) Head() (string, error) {
	l := len(t.records)
	if l > 0 {
		return t.rawRecords[l-1].Id, nil
	}
	return t.rawRoot.Id, nil
}

func (t *ACLListStorageBuilder) SetHead(headId string) error {
	panic("SetHead is not implemented")
}

func (t *ACLListStorageBuilder) Root() (*aclrecordproto2.RawACLRecordWithId, error) {
	return t.rawRoot, nil
}

func (t *ACLListStorageBuilder) GetRawRecord(ctx context.Context, id string) (*aclrecordproto2.RawACLRecordWithId, error) {
	recIdx, ok := t.indexes[id]
	if !ok {
		if id == t.rawRoot.Id {
			return t.rawRoot, nil
		}
		return nil, fmt.Errorf("no such record")
	}
	return t.rawRecords[recIdx], nil
}

func (t *ACLListStorageBuilder) AddRawRecord(ctx context.Context, rec *aclrecordproto2.RawACLRecordWithId) error {
	panic("implement me")
}

func (t *ACLListStorageBuilder) Id() string {
	return t.id
}

func (t *ACLListStorageBuilder) GetRawRecords() []*aclrecordproto2.RawACLRecordWithId {
	return t.rawRecords
}

func (t *ACLListStorageBuilder) GetKeychain() *YAMLKeychain {
	return t.keychain
}

func (t *ACLListStorageBuilder) Parse(tree *YMLList) {
	// Just to clarify - we are generating new identities for the ones that
	// are specified in the yml file, because our identities should be Ed25519
	// the same thing is happening for the encryption keys
	t.keychain.ParseKeys(&tree.Keys)
	t.parseRoot(tree.Root)
	prevId := t.id
	for idx, rec := range tree.Records {
		newRecord := t.parseRecord(rec, prevId)
		rawRecord := t.createRaw(newRecord, newRecord.Identity)
		t.records = append(t.records, newRecord)
		t.rawRecords = append(t.rawRecords, rawRecord)
		t.indexes[rawRecord.Id] = idx
		prevId = rawRecord.Id
	}
}

func (t *ACLListStorageBuilder) parseRecord(rec *Record, prevId string) *aclrecordproto2.ACLRecord {
	k := t.keychain.GetKey(rec.ReadKey).(*SymKey)
	var aclChangeContents []*aclrecordproto2.ACLContentValue
	for _, ch := range rec.AclChanges {
		aclChangeContent := t.parseACLChange(ch)
		aclChangeContents = append(aclChangeContents, aclChangeContent)
	}
	data := &aclrecordproto2.ACLData{
		AclContent: aclChangeContents,
	}
	bytes, _ := data.Marshal()

	return &aclrecordproto2.ACLRecord{
		PrevId:             prevId,
		Identity:           []byte(t.keychain.GetIdentity(rec.Identity)),
		Data:               bytes,
		CurrentReadKeyHash: k.Hash,
		Timestamp:          time.Now().Unix(),
	}
}

func (t *ACLListStorageBuilder) parseACLChange(ch *ACLChange) (convCh *aclrecordproto2.ACLContentValue) {
	switch {
	case ch.UserAdd != nil:
		add := ch.UserAdd

		encKey := t.keychain.GetKey(add.EncryptionKey).(encryptionkey.PrivKey)
		rawKey, _ := encKey.GetPublic().Raw()

		convCh = &aclrecordproto2.ACLContentValue{
			Value: &aclrecordproto2.ACLContentValue_UserAdd{
				UserAdd: &aclrecordproto2.ACLUserAdd{
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

		idKey, _ := t.keychain.SigningKeysByYAMLIdentity[join.Identity].GetPublic().Raw()
		signKey := t.keychain.GetKey(join.AcceptSignature).(signingkey.PrivKey)
		signature, err := signKey.Sign(idKey)
		if err != nil {
			panic(err)
		}

		convCh = &aclrecordproto2.ACLContentValue{
			Value: &aclrecordproto2.ACLContentValue_UserJoin{
				UserJoin: &aclrecordproto2.ACLUserJoin{
					Identity:          []byte(t.keychain.GetIdentity(join.Identity)),
					EncryptionKey:     rawKey,
					AcceptSignature:   signature,
					InviteId:          join.InviteId,
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

		convCh = &aclrecordproto2.ACLContentValue{
			Value: &aclrecordproto2.ACLContentValue_UserInvite{
				UserInvite: &aclrecordproto2.ACLUserInvite{
					AcceptPublicKey:   rawAcceptKey,
					EncryptPublicKey:  rawEncKey,
					EncryptedReadKeys: t.encryptReadKeys(invite.EncryptedReadKeys, encKey),
					Permissions:       t.convertPermission(invite.Permissions),
					InviteId:          invite.InviteId,
				},
			},
		}
	case ch.UserPermissionChange != nil:
		permissionChange := ch.UserPermissionChange

		convCh = &aclrecordproto2.ACLContentValue{
			Value: &aclrecordproto2.ACLContentValue_UserPermissionChange{
				UserPermissionChange: &aclrecordproto2.ACLUserPermissionChange{
					Identity:    []byte(t.keychain.GetIdentity(permissionChange.Identity)),
					Permissions: t.convertPermission(permissionChange.Permission),
				},
			},
		}
	case ch.UserRemove != nil:
		remove := ch.UserRemove

		newReadKey := t.keychain.GetKey(remove.NewReadKey).(*SymKey)

		var replaces []*aclrecordproto2.ACLReadKeyReplace
		for _, id := range remove.IdentitiesLeft {
			encKey := t.keychain.EncryptionKeysByYAMLIdentity[id]
			rawEncKey, _ := encKey.GetPublic().Raw()
			encReadKey, err := encKey.GetPublic().Encrypt(newReadKey.Key.Bytes())
			if err != nil {
				panic(err)
			}
			replaces = append(replaces, &aclrecordproto2.ACLReadKeyReplace{
				Identity:         []byte(t.keychain.GetIdentity(id)),
				EncryptionKey:    rawEncKey,
				EncryptedReadKey: encReadKey,
			})
		}

		convCh = &aclrecordproto2.ACLContentValue{
			Value: &aclrecordproto2.ACLContentValue_UserRemove{
				UserRemove: &aclrecordproto2.ACLUserRemove{
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

func (t *ACLListStorageBuilder) convertPermission(perm string) aclrecordproto2.ACLUserPermissions {
	switch perm {
	case "admin":
		return aclrecordproto2.ACLUserPermissions_Admin
	case "writer":
		return aclrecordproto2.ACLUserPermissions_Writer
	case "reader":
		return aclrecordproto2.ACLUserPermissions_Reader
	default:
		panic(fmt.Sprintf("incorrect permission: %s", perm))
	}
}

func (t *ACLListStorageBuilder) traverseFromHead(f func(rec *aclrecordproto2.ACLRecord, id string) error) (err error) {
	for i := len(t.records) - 1; i >= 0; i-- {
		err = f(t.records[i], t.rawRecords[i].Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *ACLListStorageBuilder) parseRoot(root *Root) {
	rawSignKey, _ := t.keychain.SigningKeysByYAMLIdentity[root.Identity].GetPublic().Raw()
	rawEncKey, _ := t.keychain.EncryptionKeysByYAMLIdentity[root.Identity].GetPublic().Raw()
	readKey, _ := aclrecordproto2.ACLReadKeyDerive(rawSignKey, rawEncKey)
	hasher := fnv.New64()
	hasher.Write(readKey.Bytes())
	t.root = &aclrecordproto2.ACLRoot{
		Identity:           rawSignKey,
		EncryptionKey:      rawEncKey,
		SpaceId:            root.SpaceId,
		EncryptedReadKey:   nil,
		DerivationScheme:   "scheme",
		CurrentReadKeyHash: hasher.Sum64(),
	}
	t.rawRoot = t.createRaw(t.root, rawSignKey)
	t.id = t.rawRoot.Id
}
