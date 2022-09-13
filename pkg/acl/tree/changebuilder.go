package tree

import (
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/common"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/symmetric"
	"github.com/gogo/protobuf/proto"
	"time"
)

var ErrEmptyChange = errors.New("change payload should not be empty")

type BuilderContent struct {
	treeHeadIds        []string
	aclHeadId          string
	snapshotBaseId     string
	currentReadKeyHash uint64
	identity           string
	isSnapshot         bool
	signingKey         signingkey.PrivKey
	readKey            *symmetric.Key
	content            []byte
}

type ChangeBuilder interface {
	ConvertFromRaw(rawIdChange *aclpb.RawTreeChangeWithId, verify bool) (ch *Change, err error)
	BuildContent(payload BuilderContent) (ch *Change, raw *aclpb.RawTreeChangeWithId, err error)
	BuildRaw(ch *Change) (*aclpb.RawTreeChangeWithId, error)
}

type changeBuilder struct {
	keys *common.Keychain
}

func newChangeBuilder(keys *common.Keychain) ChangeBuilder {
	return &changeBuilder{keys: keys}
}

func (c *changeBuilder) ConvertFromRaw(rawIdChange *aclpb.RawTreeChangeWithId, verify bool) (ch *Change, err error) {
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

	raw := &aclpb.RawTreeChange{}
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

	unmarshalled := &aclpb.TreeChange{}
	err = proto.Unmarshal(raw.Payload, unmarshalled)
	if err != nil {
		return
	}

	ch = NewChange(rawIdChange.Id, unmarshalled, raw.Signature)
	return
}

func (c *changeBuilder) BuildContent(payload BuilderContent) (ch *Change, rawIdChange *aclpb.RawTreeChangeWithId, err error) {
	change := &aclpb.TreeChange{
		TreeHeadIds:        payload.treeHeadIds,
		AclHeadId:          payload.aclHeadId,
		SnapshotBaseId:     payload.snapshotBaseId,
		CurrentReadKeyHash: payload.currentReadKeyHash,
		Timestamp:          int64(time.Now().Nanosecond()),
		Identity:           payload.identity,
		IsSnapshot:         payload.isSnapshot,
	}

	encrypted, err := payload.readKey.Encrypt(payload.content)
	if err != nil {
		return
	}
	change.ChangesData = encrypted

	marshalledChange, err := proto.Marshal(change)
	if err != nil {
		return
	}

	signature, err := payload.signingKey.Sign(marshalledChange)
	if err != nil {
		return
	}

	raw := &aclpb.RawTreeChange{
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
	ch.ParsedModel = payload.content

	rawIdChange = &aclpb.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        id,
	}
	return
}

func (c *changeBuilder) BuildRaw(ch *Change) (raw *aclpb.RawTreeChangeWithId, err error) {
	var marshalled []byte
	marshalled, err = ch.Content.Marshal()
	if err != nil {
		return
	}

	marshalledRawChange, err := proto.Marshal(&aclpb.RawTreeChange{
		Payload:   marshalled,
		Signature: ch.Sign,
	})
	if err != nil {
		return
	}

	raw = &aclpb.RawTreeChangeWithId{
		RawChange: marshalledRawChange,
		Id:        ch.Id,
	}
	return
}
