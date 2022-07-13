package treestoragebuilder

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/pb"
	testpb "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/testchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/testutils/yamltests"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage"
	storagepb "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/treestorage/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/slice"
	"io/ioutil"
	"path"

	"github.com/gogo/protobuf/proto"
	"gopkg.in/yaml.v3"
)

const plainTextDocType uint16 = 1

type treeChange struct {
	*pb.ACLChange
	id      string
	readKey *SymKey
	signKey signingkey.SigningPrivKey

	changesDataDecrypted []byte
}

type updateUseCase struct {
	changes map[string]*treeChange
}

type TreeStorageBuilder struct {
	treeId     string
	allChanges map[string]*treeChange
	updates    map[string]*updateUseCase
	heads      []string
	orphans    []string
	keychain   *Keychain
	header     *storagepb.TreeHeader
}

func NewTreeStorageBuilder(keychain *Keychain) *TreeStorageBuilder {
	return &TreeStorageBuilder{
		allChanges: make(map[string]*treeChange),
		updates:    make(map[string]*updateUseCase),
		keychain:   keychain,
	}
}

func NewTreeStorageBuilderWithTestName(name string) (*TreeStorageBuilder, error) {
	filePath := path.Join(yamltests.Path(), name)
	return NewTreeStorageBuilderFromFile(filePath)
}

func NewTreeStorageBuilderFromFile(file string) (*TreeStorageBuilder, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	ymlTree := YMLTree{}
	err = yaml.Unmarshal(content, &ymlTree)
	if err != nil {
		return nil, err
	}

	tb := NewTreeStorageBuilder(NewKeychain())
	tb.Parse(&ymlTree)

	return tb, nil
}

func (t *TreeStorageBuilder) TreeID() (string, error) {
	return t.treeId, nil
}

func (t *TreeStorageBuilder) GetKeychain() *Keychain {
	return t.keychain
}

func (t *TreeStorageBuilder) Heads() ([]string, error) {
	return t.heads, nil
}

func (t *TreeStorageBuilder) AddRawChange(change *treestorage.RawChange) error {
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

	t.allChanges[change.Id] = &treeChange{
		ACLChange:            aclChange,
		id:                   change.Id,
		readKey:              readKey,
		signKey:              signKey,
		changesDataDecrypted: changesData,
	}
	return nil
}

func (t *TreeStorageBuilder) AddOrphans(orphans ...string) error {
	t.orphans = append(t.orphans, orphans...)
	return nil
}

func (t *TreeStorageBuilder) AddChange(change aclchanges.Change) error {
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

	t.allChanges[change.CID()] = &treeChange{
		ACLChange:            aclChange,
		id:                   change.CID(),
		readKey:              readKey,
		signKey:              signKey,
		changesDataDecrypted: changesData,
	}
	return nil
}

func (t *TreeStorageBuilder) Orphans() ([]string, error) {
	return t.orphans, nil
}

func (t *TreeStorageBuilder) SetHeads(heads []string) error {
	// we should copy here instead of just setting the value
	t.heads = heads
	return nil
}

func (t *TreeStorageBuilder) RemoveOrphans(orphans ...string) error {
	t.orphans = slice.Difference(t.orphans, orphans)
	return nil
}

func (t *TreeStorageBuilder) GetChange(ctx context.Context, recordID string) (*treestorage.RawChange, error) {
	return t.getChange(recordID, t.allChanges), nil
}

func (t *TreeStorageBuilder) GetUpdates(useCase string) []*treestorage.RawChange {
	var res []*treestorage.RawChange
	update := t.updates[useCase]
	for _, ch := range update.changes {
		rawCh := t.getChange(ch.id, update.changes)
		res = append(res, rawCh)
	}
	return res
}

func (t *TreeStorageBuilder) Header() (*storagepb.TreeHeader, error) {
	return t.header, nil
}

func (t *TreeStorageBuilder) getChange(changeId string, m map[string]*treeChange) *treestorage.RawChange {
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

	transformedRec := &treestorage.RawChange{
		Payload:   aclMarshaled,
		Signature: signature,
		Id:        changeId,
	}
	return transformedRec
}

func (t *TreeStorageBuilder) Parse(tree *YMLTree) {
	// Just to clarify - we are generating new identities for the ones that
	// are specified in the yml file, because our identities should be Ed25519
	// the same thing is happening for the encryption keys
	t.keychain.ParseKeys(&tree.Keys)
	t.treeId = t.parseTreeId(tree.Description)
	for _, ch := range tree.Changes {
		newChange := t.parseChange(ch)
		t.allChanges[newChange.id] = newChange
	}

	t.parseGraph(tree)
	t.parseOrphans(tree)
	t.parseHeader(tree)
	t.parseUpdates(tree.Updates)
}

func (t *TreeStorageBuilder) parseChange(ch *Change) *treeChange {
	newChange := &treeChange{
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

func (t *TreeStorageBuilder) parseTreeId(description *TreeDescription) string {
	if description == nil {
		panic("no author in tree")
	}
	return description.Author + ".tree.id"
}

func (t *TreeStorageBuilder) parseChangeSnapshot(s *PlainTextSnapshot) *testpb.PlainTextChangeSnapshot {
	return &testpb.PlainTextChangeSnapshot{
		Text: s.Text,
	}
}

func (t *TreeStorageBuilder) parseACLSnapshot(s *ACLSnapshot) *pb.ACLChangeACLSnapshot {
	newState := &pb.ACLChangeACLState{}
	for _, state := range s.UserStates {
		aclUserState := &pb.ACLChangeUserState{}
		aclUserState.Identity = t.keychain.GetIdentity(state.Identity)

		encKey := t.keychain.
			GetKey(state.EncryptionKey).(encryptionkey.EncryptionPrivKey)
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

func (t *TreeStorageBuilder) parseDocumentChange(ch *PlainTextChange) (convCh *testpb.PlainTextChangeContent) {
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

func (t *TreeStorageBuilder) parseACLChange(ch *ACLChange) (convCh *pb.ACLChangeACLContentValue) {
	switch {
	case ch.UserAdd != nil:
		add := ch.UserAdd

		encKey := t.keychain.
			GetKey(add.EncryptionKey).(encryptionkey.EncryptionPrivKey)
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
			GetKey(join.EncryptionKey).(encryptionkey.EncryptionPrivKey)
		rawKey, _ := encKey.GetPublic().Raw()

		idKey, _ := t.keychain.SigningKeys[join.Identity].GetPublic().Raw()
		signKey := t.keychain.GetKey(join.AcceptSignature).(signingkey.SigningPrivKey)
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
		rawAcceptKey, _ := t.keychain.GetKey(invite.AcceptKey).(signingkey.SigningPrivKey).GetPublic().Raw()
		encKey := t.keychain.
			GetKey(invite.EncryptionKey).(encryptionkey.EncryptionPrivKey)
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

func (t *TreeStorageBuilder) encryptReadKeys(keys []string, encKey encryptionkey.EncryptionPrivKey) (enc [][]byte) {
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

func (t *TreeStorageBuilder) convertPermission(perm string) pb.ACLChangeUserPermissions {
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

func (t *TreeStorageBuilder) traverseFromHeads(f func(t *treeChange) error) error {
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

func (t *TreeStorageBuilder) parseUpdates(updates []*Update) {
	for _, update := range updates {
		useCase := &updateUseCase{
			changes: map[string]*treeChange{},
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

func (t *TreeStorageBuilder) parseGraph(tree *YMLTree) {
	for _, node := range tree.Graph {
		rec := t.allChanges[node.Id]
		rec.AclHeadIds = node.ACLHeads
		rec.TreeHeadIds = node.TreeHeads
		rec.SnapshotBaseId = node.BaseSnapshot
	}
}

func (t *TreeStorageBuilder) parseOrphans(tree *YMLTree) {
	t.orphans = tree.Orphans
}

func (t *TreeStorageBuilder) parseHeader(tree *YMLTree) {
	t.header = &storagepb.TreeHeader{
		FirstChangeId: tree.Header.FirstChangeId,
		IsWorkspace:   tree.Header.IsWorkspace,
	}
}
