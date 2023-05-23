package coordinatorproto

import (
	"context"
	"crypto/rand"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type fixture struct {
	networkKey      crypto.PrivKey
	accountKey      crypto.PubKey
	accountIdentity []byte
	ctx             context.Context
	originalReceipt *SpaceReceipt
	signedReceipt   *SpaceReceiptWithSignature
	spaceId         string
	peerId          string
}

func newFixture(t *testing.T) *fixture {
	networkKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	accountKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	accountKeyProto, err := accountKey.GetPublic().Marshall()
	require.NoError(t, err)
	return &fixture{
		spaceId:         "spaceId",
		peerId:          "peerId",
		accountIdentity: accountKeyProto,
		networkKey:      networkKey,
		accountKey:      accountKey.GetPublic(),
	}
}

func (fx *fixture) prepareReceipt(t *testing.T, validPeriod time.Duration) {
	var err error
	fx.signedReceipt, err = PrepareSpaceReceipt(fx.spaceId, fx.peerId, validPeriod, fx.accountKey, fx.networkKey)
	require.NoError(t, err)
	fx.originalReceipt = &SpaceReceipt{}
	err = proto.Unmarshal(fx.signedReceipt.SpaceReceiptPayload, fx.originalReceipt)
	require.NoError(t, err)
	return
}

func (fx *fixture) updateReceipt(t *testing.T, update func(t *testing.T, receipt *SpaceReceipt)) {
	update(t, fx.originalReceipt)
	marshalled, err := proto.Marshal(fx.originalReceipt)
	require.NoError(t, err)
	signature, err := fx.networkKey.Sign(marshalled)
	require.NoError(t, err)
	fx.signedReceipt = &SpaceReceiptWithSignature{
		SpaceReceiptPayload: marshalled,
		Signature:           signature,
	}
}

func TestReceiptValid(t *testing.T) {
	fx := newFixture(t)
	fx.prepareReceipt(t, time.Second)
	err := CheckReceipt(fx.peerId, fx.spaceId, fx.accountIdentity, fx.networkKey.GetPublic().Network(), fx.signedReceipt)
	require.NoError(t, err)
}

func TestReceiptIncorrectSpaceId(t *testing.T) {
	fx := newFixture(t)
	fx.prepareReceipt(t, time.Second)
	err := CheckReceipt(fx.peerId, "otherId", fx.accountIdentity, fx.networkKey.GetPublic().Network(), fx.signedReceipt)
	require.Error(t, errReceiptSpaceIdIncorrect, err)
}

func TestReceiptIncorrectPeerId(t *testing.T) {
	fx := newFixture(t)
	fx.prepareReceipt(t, time.Second)
	err := CheckReceipt("otherId", fx.spaceId, fx.accountIdentity, fx.networkKey.GetPublic().Network(), fx.signedReceipt)
	require.Error(t, errReceiptPeerIdIncorrect, err)
}

func TestReceiptIncorrectAccountIdentity(t *testing.T) {
	fx := newFixture(t)
	fx.prepareReceipt(t, time.Second)
	err := CheckReceipt(fx.peerId, fx.spaceId, []byte("some identity"), fx.networkKey.GetPublic().Network(), fx.signedReceipt)
	require.Error(t, errReceiptAccountIncorrect, err)
}

func TestReceiptIncorrectNetworkId(t *testing.T) {
	fx := newFixture(t)
	fx.prepareReceipt(t, time.Second)

	t.Run("random network id", func(t *testing.T) {
		fx.updateReceipt(t, func(t *testing.T, receipt *SpaceReceipt) {
			receipt.NetworkId = "some random network id"
		})
		err := CheckReceipt(fx.peerId, fx.spaceId, fx.accountIdentity, fx.networkKey.GetPublic().Network(), fx.signedReceipt)
		require.Error(t, err)
	})
	t.Run("random incorrect key", func(t *testing.T) {
		fx.updateReceipt(t, func(t *testing.T, receipt *SpaceReceipt) {
			randomKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
			require.NoError(t, err)
			receipt.NetworkId = randomKey.GetPublic().Network()
		})
		err := CheckReceipt(fx.peerId, fx.spaceId, fx.accountIdentity, fx.networkKey.GetPublic().Network(), fx.signedReceipt)
		require.Error(t, errNetworkIsIncorrect, err)
	})
}

func TestReceiptIncorrectSignature(t *testing.T) {
	fx := newFixture(t)
	fx.prepareReceipt(t, time.Second)
	fx.signedReceipt.Signature = []byte("random sig")
	err := CheckReceipt(fx.peerId, fx.spaceId, fx.accountIdentity, fx.networkKey.GetPublic().Network(), fx.signedReceipt)
	require.Error(t, errReceiptSignatureIncorrect, err)
}

func TestReceiptExpired(t *testing.T) {
	fx := newFixture(t)
	fx.prepareReceipt(t, time.Second)
	fx.updateReceipt(t, func(t *testing.T, receipt *SpaceReceipt) {
		receipt.ValidUntil = uint64(time.Now().Add(-time.Second).Unix())
	})
	err := CheckReceipt(fx.peerId, fx.spaceId, fx.accountIdentity, fx.networkKey.GetPublic().Network(), fx.signedReceipt)
	require.Error(t, errReceiptExpired, err)
}
