//go:generate mockgen -destination mock_credentialprovider/mock_credentialprovider.go github.com/anytypeio/any-sync/commonspace/credentialprovider CredentialProvider
package credentialprovider

import (
	"context"
	"github.com/anytypeio/any-sync/app"
	"github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	"github.com/anytypeio/any-sync/coordinator/coordinatorclient"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/gogo/protobuf/proto"
)

const CName = "common.commonspace.credentialprovider"

func New() CredentialProvider {
	return &credentialProvider{}
}

type CredentialProvider interface {
	app.Component
	GetCredential(ctx context.Context, spaceHeader *spacesyncproto.RawSpaceHeaderWithId) ([]byte, error)
}

type credentialProvider struct {
	conf   nodeconf.Service
	client coordinatorclient.CoordinatorClient
}

func (c *credentialProvider) Init(a *app.App) (err error) {
	c.conf = a.MustComponent(nodeconf.CName).(nodeconf.Service)
	c.client = a.MustComponent(coordinatorclient.CName).(coordinatorclient.CoordinatorClient)
	return
}

func (c *credentialProvider) Name() (name string) {
	return CName
}

func (c *credentialProvider) GetCredential(ctx context.Context, spaceHeader *spacesyncproto.RawSpaceHeaderWithId) ([]byte, error) {
	if c.conf.GetLast().IsResponsible(spaceHeader.Id) {
		return nil, nil
	}
	receipt, err := c.client.SpaceSign(ctx, spaceHeader.Id, spaceHeader.RawHeader)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(receipt)
}
