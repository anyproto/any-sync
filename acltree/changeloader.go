package acltree

import (
	"context"
	"fmt"
	"time"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/thread"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/keys"
	"github.com/gogo/protobuf/proto"
)

type changeLoader struct {
	cache                map[string]*Change
	identityKeys         map[string]keys.SigningPubKey
	signingPubKeyDecoder keys.SigningPubKeyDecoder
	thread               thread.Thread
	changeCreator        func(id string, ch *pb.ACLChange) *Change
}

func newChangeLoader(
	thread thread.Thread,
	signingPubKeyDecoder keys.SigningPubKeyDecoder,
	changeCreator func(id string, ch *pb.ACLChange) *Change) *changeLoader {
	return &changeLoader{
		signingPubKeyDecoder: signingPubKeyDecoder,
		thread:               thread,
		changeCreator:        changeCreator,
	}
}

func (c *changeLoader) init(cache map[string]*Change,
	identityKeys map[string]keys.SigningPubKey) {
	c.cache = cache
	c.identityKeys = identityKeys
}

func (c *changeLoader) loadChange(id string) (ch *Change, err error) {
	if ch, ok := c.cache[id]; ok {
		return ch, nil
	}

	// TODO: Add virtual changes logic
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	change, err := c.thread.GetChange(ctx, id)
	if err != nil {
		return nil, err
	}

	aclChange, err := c.makeVerifiedACLChange(change)
	if err != nil {
		return nil, err
	}

	ch = c.changeCreator(id, aclChange)
	c.cache[id] = ch

	return ch, nil
}

func (c *changeLoader) verify(identity string, payload, signature []byte) (isVerified bool, err error) {
	identityKey, exists := c.identityKeys[identity]
	if !exists {
		identityKey, err = c.signingPubKeyDecoder.DecodeFromString(identity)
		if err != nil {
			return
		}
		c.identityKeys[identity] = identityKey
	}
	return identityKey.Verify(payload, signature)
}

func (c *changeLoader) makeVerifiedACLChange(change *thread.RawChange) (aclChange *pb.ACLChange, err error) {
	aclChange = new(pb.ACLChange)

	// TODO: think what should we do with such cases, because this can be used by attacker to break our tree
	if err = proto.Unmarshal(change.Payload, aclChange); err != nil {
		return
	}
	var verified bool
	verified, err = c.verify(aclChange.Identity, change.Payload, change.Signature)
	if err != nil {
		return
	}
	if !verified {
		err = fmt.Errorf("the signature of the payload cannot be verified")
		return
	}
	return
}
