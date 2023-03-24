package objecttree

import (
	"errors"
	"github.com/anytypeio/any-sync/commonspace/object/keychain"
	"github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anytypeio/any-sync/util/cidutil"
	"github.com/anytypeio/any-sync/util/crypto"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
	"github.com/gogo/protobuf/proto"
	"time"
)

var ErrEmptyChange = errors.New("change payload should not be empty")

type BuilderContent struct {
	TreeHeadIds        []string
	AclHeadId          string
	SnapshotBaseId     string
	CurrentReadKeyHash uint64
	Identity           []byte
	IsSnapshot         bool
	SigningKey         signingkey.PrivKey
	ReadKey            *crypto.AESKey
	Content            []byte
}

type InitialContent struct {
	AclHeadId     string
	Identity      []byte
	SigningKey    signingkey.PrivKey
	SpaceId       string
	Seed          []byte
	ChangeType    string
	ChangePayload []byte
	Timestamp     int64
}

type nonVerifiableChangeBuilder struct {
	ChangeBuilder
}

func (c *nonVerifiableChangeBuilder) BuildRoot(payload InitialContent) (ch *Change, raw *treechangeproto.RawTreeChangeWithId, err error) {
	return c.ChangeBuilder.BuildRoot(payload)
}

func (c *nonVerifiableChangeBuilder) Unmarshall(rawChange *treechangeproto.RawTreeChangeWithId, verify bool) (ch *Change, err error) {
	return c.ChangeBuilder.Unmarshall(rawChange, false)
}

func (c *nonVerifiableChangeBuilder) Build(payload BuilderContent) (ch *Change, raw *treechangeproto.RawTreeChangeWithId, err error) {
	return c.ChangeBuilder.Build(payload)
}

func (c *nonVerifiableChangeBuilder) Marshall(ch *Change) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	return c.ChangeBuilder.Marshall(ch)
}

type ChangeBuilder interface {
	Unmarshall(rawIdChange *treechangeproto.RawTreeChangeWithId, verify bool) (ch *Change, err error)
	Build(payload BuilderContent) (ch *Change, raw *treechangeproto.RawTreeChangeWithId, err error)
	BuildRoot(payload InitialContent) (ch *Change, raw *treechangeproto.RawTreeChangeWithId, err error)
	Marshall(ch *Change) (*treechangeproto.RawTreeChangeWithId, error)
}

type changeBuilder struct {
	rootChange *treechangeproto.RawTreeChangeWithId
	keys       *keychain.Keychain
}

func NewChangeBuilder(keys *keychain.Keychain, rootChange *treechangeproto.RawTreeChangeWithId) ChangeBuilder {
	return &changeBuilder{keys: keys, rootChange: rootChange}
}

func (c *changeBuilder) Unmarshall(rawIdChange *treechangeproto.RawTreeChangeWithId, verify bool) (ch *Change, err error) {
	if rawIdChange.GetRawChange() == nil {
		err = ErrEmptyChange
		return
	}

	if verify {
		// verifying ID
		if !cidutil.VerifyCid(rawIdChange.RawChange, rawIdChange.Id) {
			err = ErrIncorrectCid
			return
		}
	}

	raw := &treechangeproto.RawTreeChange{} // TODO: sync pool
	err = proto.Unmarshal(rawIdChange.GetRawChange(), raw)
	if err != nil {
		return
	}
	ch, err = c.unmarshallRawChange(raw, rawIdChange.Id)
	if err != nil {
		return
	}

	if verify {
		var identityKey signingkey.PubKey
		identityKey, err = c.keys.GetOrAdd(ch.Identity)
		if err != nil {
			return
		}

		// verifying signature
		var res bool
		res, err = identityKey.Verify(raw.Payload, raw.Signature)
		if err != nil {
			return
		}
		if !res {
			err = ErrIncorrectSignature
			return
		}
	}
	return
}

func (c *changeBuilder) SetRootRawChange(rawIdChange *treechangeproto.RawTreeChangeWithId) {
	c.rootChange = rawIdChange
}

func (c *changeBuilder) BuildRoot(payload InitialContent) (ch *Change, rawIdChange *treechangeproto.RawTreeChangeWithId, err error) {
	change := &treechangeproto.RootChange{
		AclHeadId:     payload.AclHeadId,
		Timestamp:     payload.Timestamp,
		Identity:      payload.Identity,
		ChangeType:    payload.ChangeType,
		ChangePayload: payload.ChangePayload,
		SpaceId:       payload.SpaceId,
		Seed:          payload.Seed,
	}
	marshalledChange, err := proto.Marshal(change)
	if err != nil {
		return
	}
	signature, err := payload.SigningKey.Sign(marshalledChange)
	if err != nil {
		return
	}
	raw := &treechangeproto.RawTreeChange{
		Payload:   marshalledChange,
		Signature: signature,
	}
	marshalledRawChange, err := proto.Marshal(raw)
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(marshalledRawChange)
	if err != nil {
		return
	}
	ch = NewChangeFromRoot(id, change, signature)
	rawIdChange = &treechangeproto.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        id,
	}
	return
}

func (c *changeBuilder) Build(payload BuilderContent) (ch *Change, rawIdChange *treechangeproto.RawTreeChangeWithId, err error) {
	change := &treechangeproto.TreeChange{
		TreeHeadIds:        payload.TreeHeadIds,
		AclHeadId:          payload.AclHeadId,
		SnapshotBaseId:     payload.SnapshotBaseId,
		CurrentReadKeyHash: payload.CurrentReadKeyHash,
		Timestamp:          time.Now().Unix(),
		Identity:           payload.Identity,
		IsSnapshot:         payload.IsSnapshot,
	}
	if payload.ReadKey != nil {
		var encrypted []byte
		encrypted, err = payload.ReadKey.Encrypt(payload.Content)
		if err != nil {
			return
		}
		change.ChangesData = encrypted
	} else {
		change.ChangesData = payload.Content
	}
	marshalledChange, err := proto.Marshal(change)
	if err != nil {
		return
	}
	signature, err := payload.SigningKey.Sign(marshalledChange)
	if err != nil {
		return
	}
	raw := &treechangeproto.RawTreeChange{
		Payload:   marshalledChange,
		Signature: signature,
	}
	marshalledRawChange, err := proto.Marshal(raw)
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(marshalledRawChange)
	if err != nil {
		return
	}
	ch = NewChange(id, change, signature)
	rawIdChange = &treechangeproto.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        id,
	}
	return
}

func (c *changeBuilder) Marshall(ch *Change) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	if c.isRoot(ch.Id) {
		return c.rootChange, nil
	}
	treeChange := &treechangeproto.TreeChange{
		TreeHeadIds:        ch.PreviousIds,
		AclHeadId:          ch.AclHeadId,
		SnapshotBaseId:     ch.SnapshotId,
		ChangesData:        ch.Data,
		CurrentReadKeyHash: ch.ReadKeyHash,
		Timestamp:          ch.Timestamp,
		Identity:           []byte(ch.Identity),
		IsSnapshot:         ch.IsSnapshot,
	}
	var marshalled []byte
	marshalled, err = treeChange.Marshal()
	if err != nil {
		return
	}

	marshalledRawChange, err := proto.Marshal(&treechangeproto.RawTreeChange{
		Payload:   marshalled,
		Signature: ch.Signature,
	})
	if err != nil {
		return
	}

	raw = &treechangeproto.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        ch.Id,
	}
	return
}

func (c *changeBuilder) unmarshallRawChange(raw *treechangeproto.RawTreeChange, id string) (ch *Change, err error) {
	if c.isRoot(id) {
		unmarshalled := &treechangeproto.RootChange{}
		err = proto.Unmarshal(raw.Payload, unmarshalled)
		if err != nil {
			return
		}
		ch = NewChangeFromRoot(id, unmarshalled, raw.Signature)
		return
	}
	unmarshalled := &treechangeproto.TreeChange{}
	err = proto.Unmarshal(raw.Payload, unmarshalled)
	if err != nil {
		return
	}

	ch = NewChange(id, unmarshalled, raw.Signature)
	return
}

func (c *changeBuilder) isRoot(id string) bool {
	if c.rootChange != nil {
		return c.rootChange.Id == id
	}
	return false
}
