//go:generate mockgen -destination mock_credentialprovider/mock_credentialprovider.go github.com/anyproto/any-sync/commonspace/credentialprovider CredentialProvider
package credentialprovider

import (
	"context"
	"github.com/anyproto/any-sync/commonspace/spacesyncproto"
)

const CName = "common.commonspace.credentialprovider"

func NewNoOp() CredentialProvider {
	return &noOpProvider{}
}

type CredentialProvider interface {
	GetCredential(ctx context.Context, spaceHeader *spacesyncproto.RawSpaceHeaderWithId) ([]byte, error)
}

type noOpProvider struct {
}

func (n noOpProvider) GetCredential(ctx context.Context, spaceHeader *spacesyncproto.RawSpaceHeaderWithId) ([]byte, error) {
	return nil, nil
}
