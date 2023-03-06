//go:generate mockgen -destination mock_credentialprovider/mock_credentialprovider.go github.com/anytypeio/any-sync/commonspace/credentialprovider CredentialProvider
package credentialprovider

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/coordinator/coordinatorclient"
	"github.com/gogo/protobuf/proto"
)

const CName = "common.commonspace.credentialprovider"

func New() app.Component {
	return &credentialProvider{}
}

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

type credentialProvider struct {
	client coordinatorclient.CoordinatorClient
}

func (c *credentialProvider) Init(a *app.App) (err error) {
	c.client = a.MustComponent(coordinatorclient.CName).(coordinatorclient.CoordinatorClient)
	return
}

func (c *credentialProvider) Name() (name string) {
	return CName
}

func (c *credentialProvider) GetCredential(ctx context.Context, spaceHeader *spacesyncproto.RawSpaceHeaderWithId) ([]byte, error) {
	receipt, err := c.client.SpaceSign(ctx, spaceHeader.Id, spaceHeader.RawHeader)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(receipt)
}
