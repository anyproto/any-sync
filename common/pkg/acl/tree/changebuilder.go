package tree

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/common"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/pkg/acl/treechangeproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/util/keys/symmetric"
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
	ReadKey            *symmetric.Key
	Content            []byte
}

type InitialContent struct {
	AclHeadId  string
	Identity   []byte
	SigningKey signingkey.PrivKey
	SpaceId    string
	Seed       []byte
	ChangeType string
	Timestamp  int64
}

type ChangeBuilder interface {
	ConvertFromRaw(rawIdChange *treechangeproto.RawTreeChangeWithId, verify bool) (ch *Change, err error)
	BuildContent(payload BuilderContent) (ch *Change, raw *treechangeproto.RawTreeChangeWithId, err error)
	BuildInitialContent(payload InitialContent) (ch *Change, raw *treechangeproto.RawTreeChangeWithId, err error)
	BuildRaw(ch *Change) (*treechangeproto.RawTreeChangeWithId, error)
	SetRootRawChange(rawIdChange *treechangeproto.RawTreeChangeWithId)
}

type changeBuilder struct {
	rootChange *treechangeproto.RawTreeChangeWithId
	keys       *common.Keychain
}

func newChangeBuilder(keys *common.Keychain, rootChange *treechangeproto.RawTreeChangeWithId) ChangeBuilder {
	return &changeBuilder{keys: keys, rootChange: rootChange}
}

func (c *changeBuilder) ConvertFromRaw(rawIdChange *treechangeproto.RawTreeChangeWithId, verify bool) (ch *Change, err error) {
	if rawIdChange.GetRawChange() == nil {
		err = ErrEmptyChange
		return
	}

	if verify {
		// verifying ID
		if !cid.VerifyCID(rawIdChange.RawChange, rawIdChange.Id) {
			err = ErrIncorrectCID
			return
		}
	}

	raw := &treechangeproto.RawTreeChange{} // TODO: sync pool
	err = proto.Unmarshal(rawIdChange.GetRawChange(), raw)
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

	return c.unmarshallRawChange(raw, rawIdChange.Id)
}

func (c *changeBuilder) SetRootRawChange(rawIdChange *treechangeproto.RawTreeChangeWithId) {
	c.rootChange = rawIdChange
}

func (c *changeBuilder) BuildInitialContent(payload InitialContent) (ch *Change, rawIdChange *treechangeproto.RawTreeChangeWithId, err error) {
	change := &treechangeproto.RootChange{
		AclHeadId:  payload.AclHeadId,
		Timestamp:  payload.Timestamp,
		Identity:   payload.Identity,
		ChangeType: payload.ChangeType,
		SpaceId:    payload.SpaceId,
		Seed:       payload.Seed,
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

	id, err := cid.NewCIDFromBytes(marshalledRawChange)
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

func (c *changeBuilder) BuildContent(payload BuilderContent) (ch *Change, rawIdChange *treechangeproto.RawTreeChangeWithId, err error) {
	change := &treechangeproto.TreeChange{
		TreeHeadIds:        payload.TreeHeadIds,
		AclHeadId:          payload.AclHeadId,
		SnapshotBaseId:     payload.SnapshotBaseId,
		CurrentReadKeyHash: payload.CurrentReadKeyHash,
		Timestamp:          int64(time.Now().Nanosecond()),
		Identity:           payload.Identity,
		IsSnapshot:         payload.IsSnapshot,
	}

	encrypted, err := payload.ReadKey.Encrypt(payload.Content)
	if err != nil {
		return
	}
	change.ChangesData = encrypted

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

	id, err := cid.NewCIDFromBytes(marshalledRawChange)
	if err != nil {
		return
	}

	ch = NewChange(id, change, signature)
	ch.Model = payload.Content

	rawIdChange = &treechangeproto.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        id,
	}
	return
}

func (c *changeBuilder) BuildRaw(ch *Change) (raw *treechangeproto.RawTreeChangeWithId, err error) {
	if ch.Id == c.rootChange.Id {
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
	if c.rootChange.Id == id {
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
