package tree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cid"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys/symmetric"
	"github.com/gogo/protobuf/proto"
	"time"
)

const componentBuilder = "tree.changebuilder"

type BuilderContent struct {
	treeHeadIds        []string
	aclHeadId          string
	snapshotBaseId     string
	currentReadKeyHash uint64
	identity           string
	isSnapshot         bool
	signingKey         signingkey.PrivKey
	readKey            *symmetric.Key
	content            proto.Marshaler
}

type ChangeBuilder interface {
	ConvertFromRaw(rawChange *aclpb.RawChange) (ch *Change, err error)
	ConvertFromRawAndVerify(rawChange *aclpb.RawChange) (ch *Change, err error)
	BuildContent(payload BuilderContent) (ch *Change, raw *aclpb.RawChange, err error)
}

type changeBuilder struct {
	keys *keychain
}

func newChangeBuilder(keys *keychain) *changeBuilder {
	return &changeBuilder{keys: keys}
}

func (c *changeBuilder) ConvertFromRaw(rawChange *aclpb.RawChange) (ch *Change, err error) {
	unmarshalled := &aclpb.Change{}
	err = proto.Unmarshal(rawChange.Payload, unmarshalled)
	if err != nil {
		return nil, err
	}

	ch = NewChange(rawChange.Id, unmarshalled, rawChange.Signature)
	return
}

func (c *changeBuilder) ConvertFromRawAndVerify(rawChange *aclpb.RawChange) (ch *Change, err error) {
	unmarshalled := &aclpb.Change{}
	ch, err = c.ConvertFromRaw(rawChange)
	if err != nil {
		return nil, err
	}

	identityKey, err := c.keys.getOrAdd(unmarshalled.Identity)
	if err != nil {
		return
	}

	res, err := identityKey.Verify(rawChange.Payload, rawChange.Signature)
	if err != nil {
		return
	}
	if !res {
		err = ErrIncorrectSignature
	}

	return
}

func (c *changeBuilder) BuildContent(payload BuilderContent) (ch *Change, raw *aclpb.RawChange, err error) {
	aclChange := &aclpb.Change{
		TreeHeadIds:        payload.treeHeadIds,
		AclHeadId:          payload.aclHeadId,
		SnapshotBaseId:     payload.snapshotBaseId,
		CurrentReadKeyHash: payload.currentReadKeyHash,
		Timestamp:          int64(time.Now().Nanosecond()),
		Identity:           payload.identity,
		IsSnapshot:         payload.isSnapshot,
	}
	marshalledData, err := payload.content.Marshal()
	if err != nil {
		return
	}

	encrypted, err := payload.readKey.Encrypt(marshalledData)
	if err != nil {
		return
	}
	aclChange.ChangesData = encrypted

	fullMarshalledChange, err := proto.Marshal(aclChange)
	if err != nil {
		return
	}

	signature, err := payload.signingKey.Sign(fullMarshalledChange)
	if err != nil {
		return
	}

	id, err := cid.NewCIDFromBytes(fullMarshalledChange)
	if err != nil {
		return
	}

	ch = NewChange(id, aclChange, signature)
	ch.ParsedModel = payload.content

	raw = &aclpb.RawChange{
		Payload:   fullMarshalledChange,
		Signature: signature,
		Id:        id,
	}
	return
}
