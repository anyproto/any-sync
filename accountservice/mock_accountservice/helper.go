package mock_accountservice

import (
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/golang/mock/gomock"
)

func NewAccountServiceWithAccount(ctrl *gomock.Controller, acc *accountdata.AccountData) *MockService {
	mock := NewMockService(ctrl)
	mock.EXPECT().Name().Return(accountservice.CName).AnyTimes()
	mock.EXPECT().Init(gomock.Any()).AnyTimes()
	mock.EXPECT().Account().Return(acc).AnyTimes()
	return mock
}
