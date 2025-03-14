// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/paymentservice/paymentserviceclient (interfaces: AnyPpClientService)
//
// Generated by this command:
//
//	mockgen -destination=mock/mock_paymentserviceclient.go -package=mock_paymentserviceclient github.com/anyproto/any-sync/paymentservice/paymentserviceclient AnyPpClientService
//

// Package mock_paymentserviceclient is a generated GoMock package.
package mock_paymentserviceclient

import (
	context "context"
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	paymentserviceproto "github.com/anyproto/any-sync/paymentservice/paymentserviceproto"
	gomock "go.uber.org/mock/gomock"
)

// MockAnyPpClientService is a mock of AnyPpClientService interface.
type MockAnyPpClientService struct {
	ctrl     *gomock.Controller
	recorder *MockAnyPpClientServiceMockRecorder
	isgomock struct{}
}

// MockAnyPpClientServiceMockRecorder is the mock recorder for MockAnyPpClientService.
type MockAnyPpClientServiceMockRecorder struct {
	mock *MockAnyPpClientService
}

// NewMockAnyPpClientService creates a new mock instance.
func NewMockAnyPpClientService(ctrl *gomock.Controller) *MockAnyPpClientService {
	mock := &MockAnyPpClientService{ctrl: ctrl}
	mock.recorder = &MockAnyPpClientServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAnyPpClientService) EXPECT() *MockAnyPpClientServiceMockRecorder {
	return m.recorder
}

// BuySubscription mocks base method.
func (m *MockAnyPpClientService) BuySubscription(ctx context.Context, in *paymentserviceproto.BuySubscriptionRequestSigned) (*paymentserviceproto.BuySubscriptionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BuySubscription", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.BuySubscriptionResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BuySubscription indicates an expected call of BuySubscription.
func (mr *MockAnyPpClientServiceMockRecorder) BuySubscription(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BuySubscription", reflect.TypeOf((*MockAnyPpClientService)(nil).BuySubscription), ctx, in)
}

// FinalizeSubscription mocks base method.
func (m *MockAnyPpClientService) FinalizeSubscription(ctx context.Context, in *paymentserviceproto.FinalizeSubscriptionRequestSigned) (*paymentserviceproto.FinalizeSubscriptionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FinalizeSubscription", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.FinalizeSubscriptionResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FinalizeSubscription indicates an expected call of FinalizeSubscription.
func (mr *MockAnyPpClientServiceMockRecorder) FinalizeSubscription(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FinalizeSubscription", reflect.TypeOf((*MockAnyPpClientService)(nil).FinalizeSubscription), ctx, in)
}

// GetAllTiers mocks base method.
func (m *MockAnyPpClientService) GetAllTiers(ctx context.Context, in *paymentserviceproto.GetTiersRequestSigned) (*paymentserviceproto.GetTiersResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllTiers", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.GetTiersResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllTiers indicates an expected call of GetAllTiers.
func (mr *MockAnyPpClientServiceMockRecorder) GetAllTiers(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllTiers", reflect.TypeOf((*MockAnyPpClientService)(nil).GetAllTiers), ctx, in)
}

// GetSubscriptionPortalLink mocks base method.
func (m *MockAnyPpClientService) GetSubscriptionPortalLink(ctx context.Context, in *paymentserviceproto.GetSubscriptionPortalLinkRequestSigned) (*paymentserviceproto.GetSubscriptionPortalLinkResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSubscriptionPortalLink", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.GetSubscriptionPortalLinkResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSubscriptionPortalLink indicates an expected call of GetSubscriptionPortalLink.
func (mr *MockAnyPpClientServiceMockRecorder) GetSubscriptionPortalLink(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSubscriptionPortalLink", reflect.TypeOf((*MockAnyPpClientService)(nil).GetSubscriptionPortalLink), ctx, in)
}

// GetSubscriptionStatus mocks base method.
func (m *MockAnyPpClientService) GetSubscriptionStatus(ctx context.Context, in *paymentserviceproto.GetSubscriptionRequestSigned) (*paymentserviceproto.GetSubscriptionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSubscriptionStatus", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.GetSubscriptionResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSubscriptionStatus indicates an expected call of GetSubscriptionStatus.
func (mr *MockAnyPpClientServiceMockRecorder) GetSubscriptionStatus(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSubscriptionStatus", reflect.TypeOf((*MockAnyPpClientService)(nil).GetSubscriptionStatus), ctx, in)
}

// GetVerificationEmail mocks base method.
func (m *MockAnyPpClientService) GetVerificationEmail(ctx context.Context, in *paymentserviceproto.GetVerificationEmailRequestSigned) (*paymentserviceproto.GetVerificationEmailResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVerificationEmail", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.GetVerificationEmailResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVerificationEmail indicates an expected call of GetVerificationEmail.
func (mr *MockAnyPpClientServiceMockRecorder) GetVerificationEmail(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVerificationEmail", reflect.TypeOf((*MockAnyPpClientService)(nil).GetVerificationEmail), ctx, in)
}

// Init mocks base method.
func (m *MockAnyPpClientService) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockAnyPpClientServiceMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockAnyPpClientService)(nil).Init), a)
}

// IsNameValid mocks base method.
func (m *MockAnyPpClientService) IsNameValid(ctx context.Context, in *paymentserviceproto.IsNameValidRequest) (*paymentserviceproto.IsNameValidResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsNameValid", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.IsNameValidResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsNameValid indicates an expected call of IsNameValid.
func (mr *MockAnyPpClientServiceMockRecorder) IsNameValid(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsNameValid", reflect.TypeOf((*MockAnyPpClientService)(nil).IsNameValid), ctx, in)
}

// Name mocks base method.
func (m *MockAnyPpClientService) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockAnyPpClientServiceMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockAnyPpClientService)(nil).Name))
}

// VerifyAppStoreReceipt mocks base method.
func (m *MockAnyPpClientService) VerifyAppStoreReceipt(ctx context.Context, in *paymentserviceproto.VerifyAppStoreReceiptRequestSigned) (*paymentserviceproto.VerifyAppStoreReceiptResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VerifyAppStoreReceipt", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.VerifyAppStoreReceiptResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// VerifyAppStoreReceipt indicates an expected call of VerifyAppStoreReceipt.
func (mr *MockAnyPpClientServiceMockRecorder) VerifyAppStoreReceipt(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VerifyAppStoreReceipt", reflect.TypeOf((*MockAnyPpClientService)(nil).VerifyAppStoreReceipt), ctx, in)
}

// VerifyEmail mocks base method.
func (m *MockAnyPpClientService) VerifyEmail(ctx context.Context, in *paymentserviceproto.VerifyEmailRequestSigned) (*paymentserviceproto.VerifyEmailResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VerifyEmail", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.VerifyEmailResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// VerifyEmail indicates an expected call of VerifyEmail.
func (mr *MockAnyPpClientServiceMockRecorder) VerifyEmail(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VerifyEmail", reflect.TypeOf((*MockAnyPpClientService)(nil).VerifyEmail), ctx, in)
}
