package mock_transport

import (
	"github.com/anyproto/any-sync/app"
	"go.uber.org/mock/gomock"
)

func NewTransportComponent(ctrl *gomock.Controller, name string) TransportComponent {
	return TransportComponent{
		CName:         name,
		MockTransport: NewMockTransport(ctrl),
	}
}

type TransportComponent struct {
	CName string
	*MockTransport
}

func (t TransportComponent) Init(a *app.App) (err error) {
	return nil
}

func (t TransportComponent) Name() (name string) {
	return t.CName
}
