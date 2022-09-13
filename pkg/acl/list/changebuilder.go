package list

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/symmetric"
	"github.com/gogo/protobuf/proto"
	"hash/fnv"
	"time"
)

type MarshalledChange = []byte

type ACLChangeBuilder interface {
	UserAdd(identity string, encryptionKey encryptionkey.PubKey, permissions aclpb.ACLUserPermissions) error
	AddId(id string) // TODO: this is only for testing
}

type aclChangeBuilder struct {
	aclState *ACLState
	list     ACLList
	acc      *account.AccountData

	aclData     *aclpb.ACLData
	id          string
	readKey     *symmetric.Key
	readKeyHash uint64
}

func newACLChangeBuilder() *aclChangeBuilder {
	return &aclChangeBuilder{}
}

func (c *aclChangeBuilder) Init(state *ACLState, list ACLList, acc *account.AccountData) {
	c.aclState = state
	c.list = list
	c.acc = acc

	c.aclData = &aclpb.ACLData{}
	// setting read key for further encryption etc
	if state.currentReadKeyHash == 0 {
		c.readKey, _ = symmetric.NewRandom()

		hasher := fnv.New64()
		hasher.Write(c.readKey.Bytes())
		c.readKeyHash = hasher.Sum64()
	} else {
		c.readKey = c.aclState.userReadKeys[c.aclState.currentReadKeyHash]
		c.readKeyHash = c.aclState.currentReadKeyHash
	}
}

func (c *aclChangeBuilder) AddId(id string) {
	c.id = id
}

func (c *aclChangeBuilder) UserAdd(identity string, encryptionKey encryptionkey.PubKey, permissions aclpb.ACLUserPermissions) error {
	var allKeys []*symmetric.Key
	if c.aclState.currentReadKeyHash != 0 {
		for _, key := range c.aclState.userReadKeys {
			allKeys = append(allKeys, key)
		}
	} else {
		allKeys = append(allKeys, c.readKey)
	}

	var encryptedKeys [][]byte
	for _, k := range allKeys {
		res, err := encryptionKey.Encrypt(k.Bytes())
		if err != nil {
			return err
		}

		encryptedKeys = append(encryptedKeys, res)
	}
	rawKey, err := encryptionKey.Raw()
	if err != nil {
		return err
	}
	ch := &aclpb.ACLContentValue{
		Value: &aclpb.ACLContentValue_UserAdd{
			UserAdd: &aclpb.ACLUserAdd{
				Identity:          []byte(identity),
				EncryptionKey:     rawKey,
				EncryptedReadKeys: encryptedKeys,
				Permissions:       permissions,
			},
		},
	}
	c.aclData.AclContent = append(c.aclData.AclContent, ch)
	return nil
}

func (c *aclChangeBuilder) BuildAndApply() (*ACLRecord, []byte, error) {
	aclRecord := &aclpb.ACLRecord{
		PrevId:             c.list.Head().Id,
		CurrentReadKeyHash: c.readKeyHash,
		Timestamp:          int64(time.Now().Nanosecond()),
		Identity:           c.acc.Identity,
	}

	marshalledData, err := proto.Marshal(c.aclData)
	if err != nil {
		return nil, nil, err
	}
	aclRecord.Data = marshalledData
	err = c.aclState.applyRecord(aclRecord)
	if err != nil {
		return nil, nil, err
	}

	fullMarshalledChange, err := proto.Marshal(aclRecord)
	if err != nil {
		return nil, nil, err
	}
	signature, err := c.acc.SignKey.Sign(fullMarshalledChange)
	if err != nil {
		return nil, nil, err
	}
	id, err := cid.NewCIDFromBytes(fullMarshalledChange)
	if err != nil {
		return nil, nil, err
	}
	ch := NewRecord(id, aclRecord)
	ch.Model = c.aclData
	ch.Sign = signature

	return ch, fullMarshalledChange, nil
}
