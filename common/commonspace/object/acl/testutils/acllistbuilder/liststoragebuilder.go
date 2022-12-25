package acllistbuilder

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/liststorage"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/testutils/yamltests"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/cidutil"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/symmetric"
	"io/ioutil"
	"path"
	"time"

	"github.com/gogo/protobuf/proto"
	"gopkg.in/yaml.v3"
)

type ACLListStorageBuilder struct {
	liststorage.ListStorage
	keychain *YAMLKeychain
}

func NewACLListStorageBuilder(keychain *YAMLKeychain) *ACLListStorageBuilder {
	return &ACLListStorageBuilder{
		keychain: keychain,
	}
}

func NewListStorageWithTestName(name string) (liststorage.ListStorage, error) {
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

func (t *ACLListStorageBuilder) createRaw(rec proto.Marshaler, identity []byte) *aclrecordproto.RawACLRecordWithId {
	protoMarshalled, err := rec.Marshal()
	if err != nil {
		panic("should be able to marshal final acl message!")
	}

	signature, err := t.keychain.SigningKeysByRealIdentity[string(identity)].Sign(protoMarshalled)
	if err != nil {
		panic("should be able to sign final acl message!")
	}

	rawRec := &aclrecordproto.RawACLRecord{
		Payload:   protoMarshalled,
		Signature: signature,
	}

	rawMarshalled, err := proto.Marshal(rawRec)
	if err != nil {
		panic(err)
	}

	id, _ := cidutil.NewCIDFromBytes(rawMarshalled)

	return &aclrecordproto.RawACLRecordWithId{
		Payload: rawMarshalled,
		Id:      id,
	}
}

func (t *ACLListStorageBuilder) GetKeychain() *YAMLKeychain {
	return t.keychain
}

func (t *ACLListStorageBuilder) Parse(l *YMLList) {
	// Just to clarify - we are generating new identities for the ones that
	// are specified in the yml file, because our identities should be Ed25519
	// the same thing is happening for the encryption keys
	t.keychain.ParseKeys(&l.Keys)
	rawRoot := t.parseRoot(l.Root)
	var err error
	t.ListStorage, err = liststorage.NewInMemoryACLListStorage(rawRoot.Id, []*aclrecordproto.RawACLRecordWithId{rawRoot})
	if err != nil {
		panic(err)
	}
	prevId := rawRoot.Id
	for _, rec := range l.Records {
		newRecord := t.parseRecord(rec, prevId)
		rawRecord := t.createRaw(newRecord, newRecord.Identity)
		err = t.AddRawRecord(context.Background(), rawRecord)
		if err != nil {
			panic(err)
		}
		prevId = rawRecord.Id
	}
	t.SetHead(prevId)
}

func (t *ACLListStorageBuilder) parseRecord(rec *Record, prevId string) *aclrecordproto.ACLRecord {
	k := t.keychain.GetKey(rec.ReadKey).(*SymKey)
	var aclChangeContents []*aclrecordproto.ACLContentValue
	for _, ch := range rec.AclChanges {
		aclChangeContent := t.parseACLChange(ch)
		aclChangeContents = append(aclChangeContents, aclChangeContent)
	}
	data := &aclrecordproto.ACLData{
		AclContent: aclChangeContents,
	}
	bytes, _ := data.Marshal()

	return &aclrecordproto.ACLRecord{
		PrevId:             prevId,
		Identity:           []byte(t.keychain.GetIdentity(rec.Identity)),
		Data:               bytes,
		CurrentReadKeyHash: k.Hash,
		Timestamp:          time.Now().UnixNano(),
	}
}

func (t *ACLListStorageBuilder) parseACLChange(ch *ACLChange) (convCh *aclrecordproto.ACLContentValue) {
	switch {
	case ch.UserAdd != nil:
		add := ch.UserAdd

		encKey := t.keychain.GetKey(add.EncryptionKey).(encryptionkey.PrivKey)
		rawKey, _ := encKey.GetPublic().Raw()

		convCh = &aclrecordproto.ACLContentValue{
			Value: &aclrecordproto.ACLContentValue_UserAdd{
				UserAdd: &aclrecordproto.ACLUserAdd{
					Identity:          []byte(t.keychain.GetIdentity(add.Identity)),
					EncryptionKey:     rawKey,
					EncryptedReadKeys: t.encryptReadKeysWithPubKey(add.EncryptedReadKeys, encKey),
					Permissions:       t.convertPermission(add.Permission),
				},
			},
		}
	case ch.UserJoin != nil:
		join := ch.UserJoin

		encKey := t.keychain.GetKey(join.EncryptionKey).(encryptionkey.PrivKey)
		rawKey, _ := encKey.GetPublic().Raw()

		idKey, _ := t.keychain.SigningKeysByYAMLName[join.Identity].GetPublic().Raw()
		signKey := t.keychain.GetKey(join.AcceptKey).(signingkey.PrivKey)
		signature, err := signKey.Sign(idKey)
		if err != nil {
			panic(err)
		}
		acceptPubKey, _ := signKey.GetPublic().Raw()

		convCh = &aclrecordproto.ACLContentValue{
			Value: &aclrecordproto.ACLContentValue_UserJoin{
				UserJoin: &aclrecordproto.ACLUserJoin{
					Identity:          []byte(t.keychain.GetIdentity(join.Identity)),
					EncryptionKey:     rawKey,
					AcceptSignature:   signature,
					AcceptPubKey:      acceptPubKey,
					EncryptedReadKeys: t.encryptReadKeysWithPubKey(join.EncryptedReadKeys, encKey),
				},
			},
		}
	case ch.UserInvite != nil:
		invite := ch.UserInvite
		rawAcceptKey, _ := t.keychain.GetKey(invite.AcceptKey).(signingkey.PrivKey).GetPublic().Raw()
		hash := t.keychain.GetKey(invite.EncryptionKey).(*SymKey).Hash
		encKey := t.keychain.ReadKeysByHash[hash]

		convCh = &aclrecordproto.ACLContentValue{
			Value: &aclrecordproto.ACLContentValue_UserInvite{
				UserInvite: &aclrecordproto.ACLUserInvite{
					AcceptPublicKey:   rawAcceptKey,
					EncryptSymKeyHash: hash,
					EncryptedReadKeys: t.encryptReadKeysWithSymKey(invite.EncryptedReadKeys, encKey.Key),
					Permissions:       t.convertPermission(invite.Permissions),
				},
			},
		}
	case ch.UserPermissionChange != nil:
		permissionChange := ch.UserPermissionChange

		convCh = &aclrecordproto.ACLContentValue{
			Value: &aclrecordproto.ACLContentValue_UserPermissionChange{
				UserPermissionChange: &aclrecordproto.ACLUserPermissionChange{
					Identity:    []byte(t.keychain.GetIdentity(permissionChange.Identity)),
					Permissions: t.convertPermission(permissionChange.Permission),
				},
			},
		}
	case ch.UserRemove != nil:
		remove := ch.UserRemove

		newReadKey := t.keychain.GetKey(remove.NewReadKey).(*SymKey)

		var replaces []*aclrecordproto.ACLReadKeyReplace
		for _, id := range remove.IdentitiesLeft {
			encKey := t.keychain.EncryptionKeysByYAMLName[id]
			rawEncKey, _ := encKey.GetPublic().Raw()
			encReadKey, err := encKey.GetPublic().Encrypt(newReadKey.Key.Bytes())
			if err != nil {
				panic(err)
			}
			replaces = append(replaces, &aclrecordproto.ACLReadKeyReplace{
				Identity:         []byte(t.keychain.GetIdentity(id)),
				EncryptionKey:    rawEncKey,
				EncryptedReadKey: encReadKey,
			})
		}

		convCh = &aclrecordproto.ACLContentValue{
			Value: &aclrecordproto.ACLContentValue_UserRemove{
				UserRemove: &aclrecordproto.ACLUserRemove{
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

func (t *ACLListStorageBuilder) encryptReadKeysWithPubKey(keys []string, encKey encryptionkey.PrivKey) (enc [][]byte) {
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

func (t *ACLListStorageBuilder) encryptReadKeysWithSymKey(keys []string, key *symmetric.Key) (enc [][]byte) {
	for _, k := range keys {
		realKey := t.keychain.GetKey(k).(*SymKey).Key.Bytes()
		res, err := key.Encrypt(realKey)
		if err != nil {
			panic(err)
		}

		enc = append(enc, res)
	}
	return
}

func (t *ACLListStorageBuilder) convertPermission(perm string) aclrecordproto.ACLUserPermissions {
	switch perm {
	case "admin":
		return aclrecordproto.ACLUserPermissions_Admin
	case "writer":
		return aclrecordproto.ACLUserPermissions_Writer
	case "reader":
		return aclrecordproto.ACLUserPermissions_Reader
	default:
		panic(fmt.Sprintf("incorrect permission: %s", perm))
	}
}

func (t *ACLListStorageBuilder) traverseFromHead(f func(rec *aclrecordproto.ACLRecord, id string) error) (err error) {
	panic("this was removed, add if needed")
}

func (t *ACLListStorageBuilder) parseRoot(root *Root) (rawRoot *aclrecordproto.RawACLRecordWithId) {
	rawSignKey, _ := t.keychain.SigningKeysByYAMLName[root.Identity].GetPublic().Raw()
	rawEncKey, _ := t.keychain.EncryptionKeysByYAMLName[root.Identity].GetPublic().Raw()
	readKey := t.keychain.ReadKeysByYAMLName[root.Identity]
	aclRoot := &aclrecordproto.ACLRoot{
		Identity:           rawSignKey,
		EncryptionKey:      rawEncKey,
		SpaceId:            root.SpaceId,
		EncryptedReadKey:   nil,
		DerivationScheme:   "scheme",
		CurrentReadKeyHash: readKey.Hash,
	}
	return t.createRaw(aclRoot, rawSignKey)
}
