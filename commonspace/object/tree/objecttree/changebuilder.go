package objecttree

import (
	"errors"

	"github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	"github.com/anyproto/any-sync/util/cidutil"
	"github.com/anyproto/any-sync/util/crypto"
)

var ErrEmptyChange = errors.New("change payload should not be empty")

type BuilderContent struct {
	TreeHeadIds    []string
	AclHeadId      string
	SnapshotBaseId string
	ReadKeyId      string
	IsSnapshot     bool
	PrivKey        crypto.PrivKey
	ReadKey        crypto.SymKey
	Content        []byte
	Timestamp      int64
	DataType       string
}

type InitialContent struct {
	AclHeadId     string
	PrivKey       crypto.PrivKey
	SpaceId       string
	Seed          []byte
	ChangeType    string
	ChangePayload []byte
	Timestamp     int64
}

type InitialDerivedContent struct {
	SpaceId       string
	ChangeType    string
	ChangePayload []byte
}

type nonVerifiableChangeBuilder struct {
	ChangeBuilder
}

func (c *nonVerifiableChangeBuilder) Unmarshall(rawChange *treechangeproto.RawTreeChangeWithId, verify bool) (ch *Change, err error) {
	return c.ChangeBuilder.Unmarshall(rawChange, false)
}

type ChangeBuilder interface {
	Unmarshall(rawIdChange *treechangeproto.RawTreeChangeWithId, verify bool) (ch *Change, err error)
	UnmarshallReduced(rawIdChange *treechangeproto.RawTreeChangeWithId) (ch *Change, err error)
	Build(payload BuilderContent) (ch *Change, raw *treechangeproto.RawTreeChangeWithId, err error)
	BuildRoot(payload InitialContent) (ch *Change, raw *treechangeproto.RawTreeChangeWithId, err error)
	BuildDerivedRoot(payload InitialDerivedContent) (ch *Change, raw *treechangeproto.RawTreeChangeWithId, err error)
	Marshall(ch *Change) (*treechangeproto.RawTreeChangeWithId, error)
}

type changeBuilder struct {
	rootChange *treechangeproto.RawTreeChangeWithId
	keys       crypto.KeyStorage
	rawTreeCh  *treechangeproto.RawTreeChange
	hasData    bool
}

func NewChangeBuilder(keys crypto.KeyStorage, rootChange *treechangeproto.RawTreeChangeWithId) ChangeBuilder {
	return &changeBuilder{keys: keys, rootChange: rootChange, hasData: true, rawTreeCh: &treechangeproto.RawTreeChange{}}
}

func NewEmptyDataChangeBuilder(keys crypto.KeyStorage, rootChange *treechangeproto.RawTreeChangeWithId) ChangeBuilder {
	return &changeBuilder{keys: keys, rootChange: rootChange, hasData: false, rawTreeCh: &treechangeproto.RawTreeChange{}}
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

	// Signature is not safe to reuse, as it escapes changeBuilder context
	c.rawTreeCh.Signature = nil
	c.rawTreeCh.Payload = c.rawTreeCh.Payload[:0]

	raw := c.rawTreeCh
	// Use UnmarshalMerge, because proto.Unmarshal calls Reset and delete buffers before unmarshalling
	err = raw.UnmarshalVT(rawIdChange.GetRawChange())
	if err != nil {
		return
	}
	ch, err = c.unmarshallRawChange(raw, rawIdChange.Id)
	if err != nil {
		return
	}

	if verify && !ch.IsDerived {
		// verifying signature
		var res bool
		res, err = ch.Identity.Verify(raw.Payload, raw.Signature)
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

func (c *changeBuilder) UnmarshallReduced(rawIdChange *treechangeproto.RawTreeChangeWithId) (ch *Change, err error) {
	if rawIdChange.GetRawChange() == nil {
		err = ErrEmptyChange
		return
	}

	// Signature is not safe to reuse, as it escapes changeBuilder context
	c.rawTreeCh.Signature = nil
	c.rawTreeCh.Payload = c.rawTreeCh.Payload[:0]

	raw := c.rawTreeCh
	// Use UnmarshalMerge, because proto.Unmarshal calls Reset and delete buffers before unmarshalling
	err = raw.UnmarshalVT(rawIdChange.GetRawChange())
	if err != nil {
		return
	}
	ch, err = c.unmarshallReducedRawChange(raw, rawIdChange.Id)
	if err != nil {
		return
	}
	return
}

func (c *changeBuilder) BuildRoot(payload InitialContent) (ch *Change, rawIdChange *treechangeproto.RawTreeChangeWithId, err error) {
	identity, err := payload.PrivKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	change := &treechangeproto.RootChange{
		AclHeadId:     payload.AclHeadId,
		Timestamp:     payload.Timestamp,
		Identity:      identity,
		ChangeType:    payload.ChangeType,
		ChangePayload: payload.ChangePayload,
		SpaceId:       payload.SpaceId,
		Seed:          payload.Seed,
	}
	marshalledChange, err := change.MarshalVT()
	if err != nil {
		return
	}
	signature, err := payload.PrivKey.Sign(marshalledChange)
	if err != nil {
		return
	}
	raw := &treechangeproto.RawTreeChange{
		Payload:   marshalledChange,
		Signature: signature,
	}
	marshalledRawChange, err := raw.MarshalVT()
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(marshalledRawChange)
	if err != nil {
		return
	}
	ch = NewChangeFromRoot(id, payload.PrivKey.GetPublic(), change, signature, false)
	rawIdChange = &treechangeproto.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        id,
	}
	return
}

func (c *changeBuilder) BuildDerivedRoot(payload InitialDerivedContent) (ch *Change, rawIdChange *treechangeproto.RawTreeChangeWithId, err error) {
	change := &treechangeproto.RootChange{
		ChangeType:    payload.ChangeType,
		ChangePayload: payload.ChangePayload,
		SpaceId:       payload.SpaceId,
		IsDerived:     true,
	}
	marshalledChange, err := change.MarshalVT()
	if err != nil {
		return
	}
	raw := &treechangeproto.RawTreeChange{
		Payload: marshalledChange,
	}
	marshalledRawChange, err := raw.MarshalVT()
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(marshalledRawChange)
	if err != nil {
		return
	}
	ch = NewChangeFromRoot(id, nil, change, nil, true)
	rawIdChange = &treechangeproto.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        id,
	}
	return
}

func (c *changeBuilder) Build(payload BuilderContent) (ch *Change, rawIdChange *treechangeproto.RawTreeChangeWithId, err error) {
	identity, err := payload.PrivKey.GetPublic().Marshall()
	if err != nil {
		return
	}
	change := &treechangeproto.TreeChange{
		TreeHeadIds:    payload.TreeHeadIds,
		AclHeadId:      payload.AclHeadId,
		SnapshotBaseId: payload.SnapshotBaseId,
		ReadKeyId:      payload.ReadKeyId,
		Timestamp:      payload.Timestamp,
		Identity:       identity,
		IsSnapshot:     payload.IsSnapshot,
		DataType:       payload.DataType,
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
	marshalledChange, err := change.MarshalVT()
	if err != nil {
		return
	}
	signature, err := payload.PrivKey.Sign(marshalledChange)
	if err != nil {
		return
	}
	raw := &treechangeproto.RawTreeChange{
		Payload:   marshalledChange,
		Signature: signature,
	}
	marshalledRawChange, err := raw.MarshalVT()
	if err != nil {
		return
	}
	id, err := cidutil.NewCidFromBytes(marshalledRawChange)
	if err != nil {
		return
	}
	ch = &Change{
		PreviousIds: change.TreeHeadIds,
		AclHeadId:   change.AclHeadId,
		Id:          id,
		SnapshotId:  change.SnapshotBaseId,
		Timestamp:   change.Timestamp,
		ReadKeyId:   change.ReadKeyId,
		Identity:    payload.PrivKey.GetPublic(),
		Signature:   signature,
		Data:        change.ChangesData,
		DataType:    change.DataType,
		IsSnapshot:  change.IsSnapshot,
	}
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
	identity, err := ch.Identity.Marshall()
	if err != nil {
		return
	}
	treeChange := &treechangeproto.TreeChange{
		TreeHeadIds:    ch.PreviousIds,
		AclHeadId:      ch.AclHeadId,
		SnapshotBaseId: ch.SnapshotId,
		ChangesData:    ch.Data,
		ReadKeyId:      ch.ReadKeyId,
		Timestamp:      ch.Timestamp,
		Identity:       identity,
		IsSnapshot:     ch.IsSnapshot,
		DataType:       ch.DataType,
	}
	var marshalled []byte
	marshalled, err = treeChange.MarshalVT()
	if err != nil {
		return
	}

	change := treechangeproto.RawTreeChange{
		Payload:   marshalled,
		Signature: ch.Signature,
	}

	marshalledRawChange, err := change.MarshalVT()

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
	var key crypto.PubKey
	if c.isRoot(id) {
		unmarshalled := &treechangeproto.RootChange{}
		err = unmarshalled.UnmarshalVT(raw.Payload)
		if err != nil {
			return
		}
		// key will be empty for derived roots
		var key crypto.PubKey
		if !unmarshalled.IsDerived {
			key, err = c.keys.PubKeyFromProto(unmarshalled.Identity)
			if err != nil {
				return
			}
		}
		ch = NewChangeFromRoot(id, key, unmarshalled, raw.Signature, unmarshalled.IsDerived)
		return
	}
	if !c.hasData {
		change := &treechangeproto.NoDataTreeChange{}
		err = change.UnmarshalVT(raw.Payload)
		if err != nil {
			return
		}
		key, err = c.keys.PubKeyFromProto(change.Identity)
		if err != nil {
			return
		}
		ch = &Change{
			PreviousIds: change.TreeHeadIds,
			AclHeadId:   change.AclHeadId,
			Id:          id,
			SnapshotId:  change.SnapshotBaseId,
			Timestamp:   change.Timestamp,
			ReadKeyId:   change.ReadKeyId,
			Identity:    key,
			Signature:   raw.Signature,
			DataType:    change.DataType,
			IsSnapshot:  change.IsSnapshot,
		}
	} else {
		change := &treechangeproto.TreeChange{}
		err = change.UnmarshalVT(raw.Payload)
		if err != nil {
			return
		}
		key, err = c.keys.PubKeyFromProto(change.Identity)
		if err != nil {
			return
		}
		ch = &Change{
			PreviousIds: change.TreeHeadIds,
			AclHeadId:   change.AclHeadId,
			Id:          id,
			SnapshotId:  change.SnapshotBaseId,
			Timestamp:   change.Timestamp,
			ReadKeyId:   change.ReadKeyId,
			Data:        change.ChangesData,
			Identity:    key,
			Signature:   raw.Signature,
			DataType:    change.DataType,
			IsSnapshot:  change.IsSnapshot,
		}
	}
	return
}

func (c *changeBuilder) unmarshallReducedRawChange(raw *treechangeproto.RawTreeChange, id string) (ch *Change, err error) {
	if c.isRoot(id) {
		unmarshalled := &treechangeproto.RootChange{}
		err = unmarshalled.UnmarshalVT(raw.Payload)
		if err != nil {
			return
		}
		// key will be empty for derived roots
		var key crypto.PubKey
		if !unmarshalled.IsDerived {
			key, err = c.keys.PubKeyFromProto(unmarshalled.Identity)
			if err != nil {
				return
			}
		}
		ch = NewChangeFromRoot(id, key, unmarshalled, raw.Signature, unmarshalled.IsDerived)
		return
	}
	unmarshalled := &treechangeproto.ReducedTreeChange{}
	err = unmarshalled.UnmarshalVT(raw.Payload)
	if err != nil {
		return
	}
	ch = &Change{
		Id:          id,
		PreviousIds: unmarshalled.TreeHeadIds,
	}
	return
}

func (c *changeBuilder) isRoot(id string) bool {
	if c.rootChange != nil {
		return c.rootChange.Id == id
	}
	return false
}

func UnmarshallRoot(rawRoot *treechangeproto.RawTreeChangeWithId) (root *treechangeproto.RootChange, err error) {
	raw := &treechangeproto.RawTreeChange{}
	err = raw.UnmarshalVT(rawRoot.GetRawChange())
	if err != nil {
		return
	}

	root = &treechangeproto.RootChange{}
	err = root.UnmarshalVT(raw.Payload)
	if err != nil {
		return
	}
	return
}
