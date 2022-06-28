package threadbuilder

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/threadmodels"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/lib/core/smartblock"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/lib/pb/model"
	"github.com/gogo/protobuf/proto"
	"io/ioutil"
	"sort"
)

type threadRecord struct {
	*pb.ACLChange
	id      string
	logId   string
	readKey *SymKey
	signKey threadmodels.SigningPrivKey

	prevRecord  *threadRecord
	changesData *pb.ACLChangeChangeData
}

type threadLog struct {
	id      string
	owner   string
	records []*threadRecord
}

type ThreadBuilder struct {
	threadId   string
	logs       map[string]*threadLog
	allRecords map[string]*threadRecord
	keychain   *Keychain
}

func NewThreadBuilder(keychain *Keychain) *ThreadBuilder {
	return &ThreadBuilder{
		logs:       make(map[string]*threadLog),
		allRecords: make(map[string]*threadRecord),
		keychain:   keychain,
	}
}

func NewThreadBuilderFromFile(file string) (*ThreadBuilder, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	thread := YMLThread{}
	err = yaml.Unmarshal(content, &thread)
	if err != nil {
		return nil, err
	}

	tb := NewThreadBuilder(NewKeychain())
	tb.Parse(&thread)

	return tb, nil
}

func (t *ThreadBuilder) ID() string {
	return t.threadId
}

func (t *ThreadBuilder) GetKeychain() *Keychain {
	return t.keychain
}

func (t *ThreadBuilder) GetLogs() ([]threadmodels.ThreadLog, error) {
	var logs []threadmodels.ThreadLog
	for _, l := range t.logs {
		logs = append(logs, threadmodels.ThreadLog{
			ID:      l.id,
			Head:    l.records[len(l.records)-1].id,
			Counter: int64(len(l.records)),
		})
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].ID < logs[j].ID
	})
	return logs, nil
}

func (t *ThreadBuilder) GetRecord(ctx context.Context, recordID string) (*threadmodels.ThreadRecord, error) {
	rec := t.allRecords[recordID]
	prevId := ""
	if rec.prevRecord != nil {
		prevId = rec.prevRecord.id
	}

	var encrypted []byte
	if rec.changesData != nil {
		m, err := proto.Marshal(rec.changesData)
		if err != nil {
			panic("should be able to marshal data!")
		}

		encrypted, err = rec.readKey.Key.Encrypt(m)
		if err != nil {
			panic("should be able to encrypt data with read key!")
		}

		rec.ChangesData = encrypted
	}

	aclMarshaled, err := proto.Marshal(rec.ACLChange)
	if err != nil {
		panic("should be able to marshal final acl message!")
	}

	signature, err := rec.signKey.Sign(aclMarshaled)
	if err != nil {
		panic("should be able to sign final acl message!")
	}

	transformedRec := &threadmodels.ThreadRecord{
		PrevId: prevId,
		Id:     rec.id,
		LogId:  rec.logId,
		Signed: &threadmodels.SignedPayload{
			Payload:   aclMarshaled,
			Signature: signature,
		},
	}
	return transformedRec, nil
}

func (t *ThreadBuilder) PushRecord(payload proto.Marshaler) (id string, err error) {
	panic("implement me")
}

func (t *ThreadBuilder) Parse(thread *YMLThread) {
	// Just to clarify - we are generating new identities for the ones that
	// are specified in the yml file, because our identities should be Ed25519
	// the same thing is happening for the encryption keys
	t.keychain.ParseKeys(&thread.Keys)
	t.threadId = t.parseThreadId(thread.Description)
	for _, l := range thread.Logs {
		newLog := &threadLog{
			id:    l.Id,
			owner: t.keychain.GetIdentity(l.Identity),
		}
		var records []*threadRecord
		for _, r := range l.Records {
			newRecord := &threadRecord{
				id:    r.Id,
				logId: newLog.id,
			}
			if len(records) > 0 {
				newRecord.prevRecord = records[len(records)-1]
			}
			k := t.keychain.GetKey(r.ReadKey).(*SymKey)
			newRecord.readKey = k
			newRecord.signKey = t.keychain.SigningKeys[l.Identity]

			aclChange := &pb.ACLChange{}
			aclChange.Identity = newLog.owner
			if len(r.AclChanges) > 0 || r.AclSnapshot != nil {
				aclChange.AclData = &pb.ACLChangeACLData{}
				if r.AclSnapshot != nil {
					aclChange.AclData.AclSnapshot = t.parseACLSnapshot(r.AclSnapshot)
				}
				if r.AclChanges != nil {
					var aclChangeContents []*pb.ACLChangeACLContentValue
					for _, ch := range r.AclChanges {
						aclChangeContent := t.parseACLChange(ch)
						aclChangeContents = append(aclChangeContents, aclChangeContent)
					}
					aclChange.AclData.AclContent = aclChangeContents
				}
			}
			if len(r.Changes) > 0 || r.Snapshot != nil {
				newRecord.changesData = &pb.ACLChangeChangeData{}
				if r.Snapshot != nil {
					newRecord.changesData.Snapshot = t.parseChangeSnapshot(r.Snapshot)
				}
				if len(r.Changes) > 0 {
					var changeContents []*pb.ChangeContent
					for _, ch := range r.Changes {
						aclChangeContent := t.parseDocumentChange(ch)
						changeContents = append(changeContents, aclChangeContent)
					}
					newRecord.changesData.Content = changeContents
				}
			}
			aclChange.CurrentReadKeyHash = k.Hash
			newRecord.ACLChange = aclChange
			t.allRecords[newRecord.id] = newRecord
			records = append(records, newRecord)
		}
		newLog.records = records
		t.logs[newLog.id] = newLog
	}
	t.parseGraph(thread)
}

func (t *ThreadBuilder) parseThreadId(description *ThreadDescription) string {
	if description == nil {
		panic("no author in thread")
	}
	key := t.keychain.SigningKeys[description.Author]
	id, err := threadmodels.CreateACLThreadID(key.GetPublic(), smartblock.SmartBlockTypeWorkspace)
	if err != nil {
		panic(err)
	}

	return id.String()
}

func (t *ThreadBuilder) parseChangeSnapshot(s *ChangeSnapshot) *pb.ChangeSnapshot {
	data := &model.SmartBlockSnapshotBase{}
	var blocks []*model.Block
	for _, b := range s.Blocks {
		modelBlock := &model.Block{Id: b.Id, ChildrenIds: b.ChildrenIds}
		blocks = append(blocks, modelBlock)
	}
	data.Blocks = blocks
	return &pb.ChangeSnapshot{
		Data: data,
	}
}

func (t *ThreadBuilder) parseACLSnapshot(s *ACLSnapshot) *pb.ACLChangeACLSnapshot {
	newState := &pb.ACLChangeACLState{}
	for _, state := range s.UserStates {
		aclUserState := &pb.ACLChangeUserState{}
		aclUserState.Identity = t.keychain.GetIdentity(state.Identity)

		encKey := t.keychain.
			GetKey(state.EncryptionKey).(threadmodels.EncryptionPrivKey)
		rawKey, _ := encKey.GetPublic().Raw()
		aclUserState.EncryptionKey = rawKey

		aclUserState.EncryptedReadKeys = t.encryptReadKeys(state.EncryptedReadKeys, encKey)
		aclUserState.Permissions = t.convertPermission(state.Permissions)
		newState.UserStates = append(newState.UserStates, aclUserState)
	}
	return &pb.ACLChangeACLSnapshot{
		AclState: newState,
	}
}

func (t *ThreadBuilder) parseDocumentChange(ch *DocumentChange) (convCh *pb.ChangeContent) {
	switch {
	case ch.BlockAdd != nil:
		blockAdd := ch.BlockAdd

		convCh = &pb.ChangeContent{
			Value: &pb.ChangeContentValueOfBlockCreate{
				BlockCreate: &pb.ChangeBlockCreate{
					TargetId: blockAdd.TargetId,
					Position: model.Block_Inner,
					Blocks:   []*model.Block{&model.Block{Id: blockAdd.Id}},
				}}}
	}
	if convCh == nil {
		panic("cannot have empty document change")
	}

	return convCh
}

func (t *ThreadBuilder) parseACLChange(ch *ACLChange) (convCh *pb.ACLChangeACLContentValue) {
	switch {
	case ch.UserAdd != nil:
		add := ch.UserAdd

		encKey := t.keychain.
			GetKey(add.EncryptionKey).(threadmodels.EncryptionPrivKey)
		rawKey, _ := encKey.GetPublic().Raw()

		convCh = &pb.ACLChangeACLContentValue{
			Value: &pb.ACLChangeACLContentValueValueOfUserAdd{
				UserAdd: &pb.ACLChangeUserAdd{
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
			GetKey(join.EncryptionKey).(threadmodels.EncryptionPrivKey)
		rawKey, _ := encKey.GetPublic().Raw()

		idKey, _ := t.keychain.SigningKeys[join.Identity].GetPublic().Raw()
		signKey := t.keychain.GetKey(join.AcceptSignature).(threadmodels.SigningPrivKey)
		signature, err := signKey.Sign(idKey)
		if err != nil {
			panic(err)
		}

		convCh = &pb.ACLChangeACLContentValue{
			Value: &pb.ACLChangeACLContentValueValueOfUserJoin{
				UserJoin: &pb.ACLChangeUserJoin{
					Identity:           t.keychain.GetIdentity(join.Identity),
					EncryptionKey:      rawKey,
					AcceptSignature:    signature,
					UserInviteChangeId: join.InviteId,
					EncryptedReadKeys:  t.encryptReadKeys(join.EncryptedReadKeys, encKey),
				},
			},
		}
	case ch.UserInvite != nil:
		invite := ch.UserInvite
		rawAcceptKey, _ := t.keychain.GetKey(invite.AcceptKey).(threadmodels.SigningPrivKey).GetPublic().Raw()
		encKey := t.keychain.
			GetKey(invite.EncryptionKey).(threadmodels.EncryptionPrivKey)
		rawEncKey, _ := encKey.GetPublic().Raw()

		convCh = &pb.ACLChangeACLContentValue{
			Value: &pb.ACLChangeACLContentValueValueOfUserInvite{
				UserInvite: &pb.ACLChangeUserInvite{
					AcceptPublicKey:   rawAcceptKey,
					EncryptPublicKey:  rawEncKey,
					EncryptedReadKeys: t.encryptReadKeys(invite.EncryptedReadKeys, encKey),
					Permissions:       t.convertPermission(invite.Permissions),
				},
			},
		}
	case ch.UserConfirm != nil:
		confirm := ch.UserConfirm

		convCh = &pb.ACLChangeACLContentValue{
			Value: &pb.ACLChangeACLContentValueValueOfUserConfirm{
				UserConfirm: &pb.ACLChangeUserConfirm{
					Identity:  t.keychain.GetIdentity(confirm.Identity),
					UserAddId: confirm.UserAddId,
				},
			},
		}
	case ch.UserPermissionChange != nil:
		permissionChange := ch.UserPermissionChange

		convCh = &pb.ACLChangeACLContentValue{
			Value: &pb.ACLChangeACLContentValueValueOfUserPermissionChange{
				UserPermissionChange: &pb.ACLChangeUserPermissionChange{
					Identity:    t.keychain.GetIdentity(permissionChange.Identity),
					Permissions: t.convertPermission(permissionChange.Permission),
				},
			},
		}
	case ch.UserRemove != nil:
		remove := ch.UserRemove

		newReadKey := t.keychain.GetKey(remove.NewReadKey).(*SymKey)

		var replaces []*pb.ACLChangeReadKeyReplace
		for _, id := range remove.IdentitiesLeft {
			identity := t.keychain.GetIdentity(id)
			encKey := t.keychain.EncryptionKeys[id]
			rawEncKey, _ := encKey.GetPublic().Raw()
			encReadKey, err := encKey.GetPublic().Encrypt(newReadKey.Key.Bytes())
			if err != nil {
				panic(err)
			}
			replaces = append(replaces, &pb.ACLChangeReadKeyReplace{
				Identity:         identity,
				EncryptionKey:    rawEncKey,
				EncryptedReadKey: encReadKey,
			})
		}

		convCh = &pb.ACLChangeACLContentValue{
			Value: &pb.ACLChangeACLContentValueValueOfUserRemove{
				UserRemove: &pb.ACLChangeUserRemove{
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

func (t *ThreadBuilder) encryptReadKeys(keys []string, encKey threadmodels.EncryptionPrivKey) (enc [][]byte) {
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

func (t *ThreadBuilder) convertPermission(perm string) pb.ACLChangeUserPermissions {
	switch perm {
	case "admin":
		return pb.ACLChange_Admin
	case "writer":
		return pb.ACLChange_Writer
	case "reader":
		return pb.ACLChange_Reader
	default:
		panic(fmt.Sprintf("incorrect permission: %s", perm))
	}
}

func (t *ThreadBuilder) traverseFromHeads(f func(t *threadRecord) error) error {
	allLogs, err := t.GetLogs()
	if err != nil {
		return err
	}

	for _, log := range allLogs {
		head := t.allRecords[log.Head]
		err = f(head)
		if err != nil {
			return err
		}

		for head.prevRecord != nil {
			head = head.prevRecord
			err = f(head)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *ThreadBuilder) parseGraph(thread *YMLThread) {
	for _, node := range thread.Graph {
		rec := t.allRecords[node.Id]
		rec.AclHeadIds = node.ACLHeads
		rec.TreeHeadIds = node.TreeHeads
		rec.SnapshotBaseId = node.BaseSnapshot
	}
}
