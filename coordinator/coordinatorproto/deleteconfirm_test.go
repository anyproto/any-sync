package coordinatorproto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDeleteConfirmation(t *testing.T) {
	fx := newFixture(t)
	delConfirm, err := PrepareDeleteConfirmation(fx.accountPrivKey, fx.spaceId, fx.peerId, fx.networkKey.GetPublic().Network())
	require.NoError(t, err)
	err = ValidateDeleteConfirmation(fx.accountKey, fx.spaceId, fx.networkKey.GetPublic().Network(), delConfirm)
	require.NoError(t, err)
}

func TestValidateAccountDeleteConfirmation(t *testing.T) {
	fx := newFixture(t)
	delConfirm, err := PrepareAccountDeleteConfirmation(fx.accountPrivKey, fx.peerId, fx.networkKey.GetPublic().Network())
	require.NoError(t, err)
	err = ValidateAccountDeleteConfirmation(fx.accountKey, fx.spaceId, fx.networkKey.GetPublic().Network(), delConfirm)
	require.NoError(t, err)
}
