package threadbuilder

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/testutils/yamltests"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"io/ioutil"
	"path"

	"github.com/gogo/protobuf/proto"
	"gopkg.in/yaml.v3"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb"
	testpb "github.com/anytypeio/go-anytype-infrastructure-experiments/testutils/testchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/thread"
	threadpb "github.com/anytypeio/go-anytype-infrastructure-experiments/thread/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
)

const plainTextDocType uint16 = 1

type threadChange struct {
	*pb.ACLChange
	id      string
	readKey *SymKey
	signKey keys.SigningPrivKey

	changesDataDecrypted []byte
}

type updateUseCase struct {
	changes map[string]*threadChange
}

type ThreadBuilder struct {
	threadId   string
	allChanges map[string]*threadChange
	updates    map[string]*updateUseCase
	heads      []string
	orphans    []string
	keychain   *Keychain
	header     *threadpb.ThreadHeader
}

func NewThreadBuilder(keychain *Keychain) *ThreadBuilder {
	return &ThreadBuilder{
		allChanges: make(map[string]*threadChange),
		updates:    make(map[string]*updateUseCase),
		keychain:   keychain,
	}
}

func NewThreadBuilderWithTestName(name string) (*ThreadBuilder, error) {
	filePath := path.Join(yamltests.Path(), name)
	return NewThreadBuilderFromFile(filePath)
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

func (t *ThreadBuilder) Heads() []string {
	return t.heads
}

func (t *ThreadBuilder) AddRawChange(change *thread.RawChange) error {
	aclChange := new(pb.ACLChange)
	var err error

	if err = proto.Unmarshal(change.Payload, aclChange); err != nil {
		return fmt.Errorf("could not unmarshall changes")
	}
	var changesData []byte

	// get correct readkey
	readKey := t.keychain.ReadKeysByHash[aclChange.CurrentReadKeyHash]
	if aclChange.ChangesData != nil {
		changesData, err = readKey.Key.Decrypt(aclChange.ChangesData)
		if err != nil {
			return fmt.Errorf("failed to decrypt changes data: %w", err)
		}
	}

	// get correct signing key
	signKey := t.keychain.SigningKeysByIdentity[aclChange.Identity]

	t.allChanges[change.Id] = &threadChange{
		ACLChange:            aclChange,
		id:                   change.Id,
		readKey:              readKey,
		signKey:              signKey,
		changesDataDecrypted: changesData,
	}
	return nil
}

func (t *ThreadBuilder) AddOrphans(orphans ...string) {
	t.orphans = append(t.orphans, orphans...)
}

func (t *ThreadBuilder) AddChange(change aclchanges.Change) error {
	aclChange := change.ProtoChange()
	var err error
	var changesData []byte

	// get correct readkey
	readKey := t.keychain.ReadKeysByHash[aclChange.CurrentReadKeyHash]
	if aclChange.ChangesData != nil {
		changesData, err = readKey.Key.Decrypt(aclChange.ChangesData)
		if err != nil {
			return fmt.Errorf("failed to decrypt changes data: %w", err)
		}
	}

	// get correct signing key
	signKey := t.keychain.SigningKeysByIdentity[aclChange.Identity]

	t.allChanges[change.CID()] = &threadChange{
		ACLChange:            aclChange,
		id:                   change.CID(),
		readKey:              readKey,
		signKey:              signKey,
		changesDataDecrypted: changesData,
	}
	return nil
}

func (t *ThreadBuilder) Orphans() []string {
	return t.orphans
}

func (t *ThreadBuilder) SetHeads(heads []string) {
	// we should copy here instead of just setting the value
	t.heads = heads
}

func (t *ThreadBuilder) RemoveOrphans(orphans ...string) {
	t.orphans = slice.Difference(t.orphans, orphans)
}

func (t *ThreadBuilder) GetChange(ctx context.Context, recordID string) (*thread.RawChange, error) {
	return t.getChange(recordID, t.allChanges), nil
}

func (t *ThreadBuilder) GetUpdates(useCase string) []*thread.RawChange {
	var res []*thread.RawChange
	update := t.updates[useCase]
	for _, ch := range update.changes {
		rawCh := t.getChange(ch.id, update.changes)
		res = append(res, rawCh)
	}
	return res
}

func (t *ThreadBuilder) Header() *threadpb.ThreadHeader {
	return t.header
}

func (t *ThreadBuilder) getChange(changeId string, m map[string]*threadChange) *thread.RawChange {
	rec := m[changeId]

	if rec.changesDataDecrypted != nil {
		encrypted, err := rec.readKey.Key.Encrypt(rec.changesDataDecrypted)
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

	transformedRec := &thread.RawChange{
		Payload:   aclMarshaled,
		Signature: signature,
		Id:        changeId,
	}
	return transformedRec
}

func (t *ThreadBuilder) Parse(thread *YMLThread) {
	// Just to clarify - we are generating new identities for the ones that
	// are specified in the yml file, because our identities should be Ed25519
	// the same thing is happening for the encryption keys
	t.keychain.ParseKeys(&thread.Keys)
	t.threadId = t.parseThreadId(thread.Description)
	for _, ch := range thread.Changes {
		newChange := t.parseChange(ch)
		t.allChanges[newChange.id] = newChange
	}

	t.parseGraph(thread)
	t.parseOrphans(thread)
	t.parseHeader(thread)
	t.parseUpdates(thread.Updates)
}

func (t *ThreadBuilder) parseChange(ch *Change) *threadChange {
	newChange := &threadChange{
		id: ch.Id,
	}
	k := t.keychain.GetKey(ch.ReadKey).(*SymKey)
	newChange.readKey = k
	newChange.signKey = t.keychain.SigningKeys[ch.Identity]
	aclChange := &pb.ACLChange{}
	aclChange.Identity = t.keychain.GetIdentity(ch.Identity)
	if len(ch.AclChanges) > 0 || ch.AclSnapshot != nil {
		aclChange.AclData = &pb.ACLChangeACLData{}
		if ch.AclSnapshot != nil {
			aclChange.AclData.AclSnapshot = t.parseACLSnapshot(ch.AclSnapshot)
		}
		if ch.AclChanges != nil {
			var aclChangeContents []*pb.ACLChangeACLContentValue
			for _, ch := range ch.AclChanges {
				aclChangeContent := t.parseACLChange(ch)
				aclChangeContents = append(aclChangeContents, aclChangeContent)
			}
			aclChange.AclData.AclContent = aclChangeContents
		}
	}
	if len(ch.Changes) > 0 || ch.Snapshot != nil {
		changesData := &testpb.PlainTextChangeData{}
		if ch.Snapshot != nil {
			changesData.Snapshot = t.parseChangeSnapshot(ch.Snapshot)
		}
		if len(ch.Changes) > 0 {
			var changeContents []*testpb.PlainTextChangeContent
			for _, ch := range ch.Changes {
				aclChangeContent := t.parseDocumentChange(ch)
				changeContents = append(changeContents, aclChangeContent)
			}
			changesData.Content = changeContents
		}
		m, err := proto.Marshal(changesData)
		if err != nil {
			return nil
		}
		newChange.changesDataDecrypted = m
	}
	aclChange.CurrentReadKeyHash = k.Hash
	newChange.ACLChange = aclChange
	return newChange
}

func (t *ThreadBuilder) parseThreadId(description *ThreadDescription) string {
	if description == nil {
		panic("no author in thread")
	}
	key := t.keychain.SigningKeys[description.Author]
	id, err := thread.CreateACLThreadID(key.GetPublic(), plainTextDocType)
	if err != nil {
		panic(err)
	}

	return id.String()
}

func (t *ThreadBuilder) parseChangeSnapshot(s *PlainTextSnapshot) *testpb.PlainTextChangeSnapshot {
	return &testpb.PlainTextChangeSnapshot{
		Text: s.Text,
	}
}

func (t *ThreadBuilder) parseACLSnapshot(s *ACLSnapshot) *pb.ACLChangeACLSnapshot {
	newState := &pb.ACLChangeACLState{}
	for _, state := range s.UserStates {
		aclUserState := &pb.ACLChangeUserState{}
		aclUserState.Identity = t.keychain.GetIdentity(state.Identity)

		encKey := t.keychain.
			GetKey(state.EncryptionKey).(keys.EncryptionPrivKey)
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

func (t *ThreadBuilder) parseDocumentChange(ch *PlainTextChange) (convCh *testpb.PlainTextChangeContent) {
	switch {
	case ch.TextAppend != nil:
		convCh = &testpb.PlainTextChangeContent{
			Value: &testpb.PlainTextChangeContentValueOfTextAppend{
				TextAppend: &testpb.PlainTextChangeTextAppend{
					Text: ch.TextAppend.Text,
				},
			},
		}
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
			GetKey(add.EncryptionKey).(keys.EncryptionPrivKey)
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
			GetKey(join.EncryptionKey).(keys.EncryptionPrivKey)
		rawKey, _ := encKey.GetPublic().Raw()

		idKey, _ := t.keychain.SigningKeys[join.Identity].GetPublic().Raw()
		signKey := t.keychain.GetKey(join.AcceptSignature).(keys.SigningPrivKey)
		signature, err := signKey.Sign(idKey)
		if err != nil {
			panic(err)
		}

		convCh = &pb.ACLChangeACLContentValue{
			Value: &pb.ACLChangeACLContentValueValueOfUserJoin{
				UserJoin: &pb.ACLChangeUserJoin{
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
		rawAcceptKey, _ := t.keychain.GetKey(invite.AcceptKey).(keys.SigningPrivKey).GetPublic().Raw()
		encKey := t.keychain.
			GetKey(invite.EncryptionKey).(keys.EncryptionPrivKey)
		rawEncKey, _ := encKey.GetPublic().Raw()

		convCh = &pb.ACLChangeACLContentValue{
			Value: &pb.ACLChangeACLContentValueValueOfUserInvite{
				UserInvite: &pb.ACLChangeUserInvite{
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

func (t *ThreadBuilder) encryptReadKeys(keys []string, encKey keys.EncryptionPrivKey) (enc [][]byte) {
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

func (t *ThreadBuilder) traverseFromHeads(f func(t *threadChange) error) error {
	uniqMap := map[string]struct{}{}
	stack := make([]string, len(t.orphans), 10)
	copy(stack, t.orphans)
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, exists := uniqMap[id]; exists {
			continue
		}

		ch := t.allChanges[id]
		uniqMap[id] = struct{}{}
		if err := f(ch); err != nil {
			return err
		}

		for _, prev := range ch.ACLChange.TreeHeadIds {
			stack = append(stack, prev)
		}
	}
	return nil
}

func (t *ThreadBuilder) parseUpdates(updates []*Update) {
	for _, update := range updates {
		useCase := &updateUseCase{
			changes: map[string]*threadChange{},
		}
		for _, ch := range update.Changes {
			newChange := t.parseChange(ch)
			useCase.changes[newChange.id] = newChange
		}
		for _, node := range update.Graph {
			rec := useCase.changes[node.Id]
			rec.AclHeadIds = node.ACLHeads
			rec.TreeHeadIds = node.TreeHeads
			rec.SnapshotBaseId = node.BaseSnapshot
		}

		t.updates[update.UseCase] = useCase
	}
}

func (t *ThreadBuilder) parseGraph(thread *YMLThread) {
	for _, node := range thread.Graph {
		rec := t.allChanges[node.Id]
		rec.AclHeadIds = node.ACLHeads
		rec.TreeHeadIds = node.TreeHeads
		rec.SnapshotBaseId = node.BaseSnapshot
	}
}

func (t *ThreadBuilder) parseOrphans(thread *YMLThread) {
	t.orphans = thread.Orphans
}

func (t *ThreadBuilder) parseHeader(thread *YMLThread) {
	t.header = &threadpb.ThreadHeader{
		FirstChangeId: thread.Header.FirstChangeId,
		IsWorkspace:   thread.Header.IsWorkspace,
	}
}
