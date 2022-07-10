package acltree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/gogo/protobuf/proto"
)

type MarshalledChange = []byte

type ACLChangeBuilder interface {
	UserAdd(identity string, encryptionKey keys.EncryptionPubKey)
	AddId(id string)      // TODO: this is only for testing
	SetMakeSnapshot(bool) // TODO: who should decide this? probably ACLTree so we can delete it
}

type ChangeBuilder interface {
	ACLChangeBuilder
	AddChangeContent(marshaler proto.Marshaler) // user code should be responsible for making regular snapshots
}

type changeBuilder struct {
	aclState *ACLState
	tree     *Tree
	acc      *account.AccountData

	aclData       *pb.ACLChangeACLData
	changeContent proto.Marshaler
	id            string
	makeSnapshot  bool
}

func newChangeBuilder() *changeBuilder {
	return &changeBuilder{}
}

func (c *changeBuilder) Init(state *ACLState, tree *Tree, acc *account.AccountData) {
	c.aclState = state
	c.tree = tree
	c.acc = acc

	c.aclData = &pb.ACLChangeACLData{}
}

func (c *changeBuilder) AddId(id string) {
	c.id = id
}

func (c *changeBuilder) SetMakeSnapshot(b bool) {
	c.makeSnapshot = b
}

func (c *changeBuilder) UserAdd(identity string, encryptionKey keys.EncryptionPubKey) {
	//TODO implement me
	panic("implement me")
}

func (c *changeBuilder) Build() (*Change, []byte, error) {
	marshalled, err := c.changeContent.Marshal()
	if err != nil {
		return nil, nil, err
	}

	encrypted, err := c.aclState.userReadKeys[c.aclState.currentReadKeyHash].
		Encrypt(marshalled)
	if err != nil {
		return nil, nil, err
	}

	aclChange := &pb.ACLChange{
		TreeHeadIds:        c.tree.Heads(),
		AclHeadIds:         c.tree.ACLHeads(),
		SnapshotBaseId:     c.tree.RootId(), // TODO: add logic for ACL snapshot
		AclData:            c.aclData,
		ChangesData:        encrypted,
		CurrentReadKeyHash: c.aclState.currentReadKeyHash,
		Timestamp:          0,
		Identity:           c.acc.Identity,
	}

	// TODO: add CID creation logic based on content
	ch := NewChange(c.id, aclChange)
	ch.DecryptedDocumentChange = marshalled

	fullMarshalledChange, err := proto.Marshal(aclChange)
	if err != nil {
		return nil, nil, err
	}
	signature, err := c.acc.SignKey.Sign(fullMarshalledChange)
	if err != nil {
		return nil, nil, err
	}
	ch.Sign = signature

	return ch, fullMarshalledChange, nil
}

func (c *changeBuilder) AddChangeContent(marshaler proto.Marshaler) {
	c.changeContent = marshaler
}
