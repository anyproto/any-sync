// Code generated by MockGen. DO NOT EDIT.
// Source: nameservice/nameserviceclient/nameserviceclient.go
//
// Generated by this command:
//
//	mockgen -source nameservice/nameserviceclient/nameserviceclient.go
//
// Package mock_nameserviceclient is a generated GoMock package.
package mock_nameserviceclient

import (
	context "context"
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	nameserviceproto "github.com/anyproto/any-sync/nameservice/nameserviceproto"
	gomock "go.uber.org/mock/gomock"
)

// MockAnyNsClientServiceBase is a mock of AnyNsClientServiceBase interface.
type MockAnyNsClientServiceBase struct {
	ctrl     *gomock.Controller
	recorder *MockAnyNsClientServiceBaseMockRecorder
}

// MockAnyNsClientServiceBaseMockRecorder is the mock recorder for MockAnyNsClientServiceBase.
type MockAnyNsClientServiceBaseMockRecorder struct {
	mock *MockAnyNsClientServiceBase
}

// NewMockAnyNsClientServiceBase creates a new mock instance.
func NewMockAnyNsClientServiceBase(ctrl *gomock.Controller) *MockAnyNsClientServiceBase {
	mock := &MockAnyNsClientServiceBase{ctrl: ctrl}
	mock.recorder = &MockAnyNsClientServiceBaseMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAnyNsClientServiceBase) EXPECT() *MockAnyNsClientServiceBaseMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockAnyNsClientServiceBase) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockAnyNsClientServiceBaseMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockAnyNsClientServiceBase)(nil).Close), ctx)
}

// GetNameByAddress mocks base method.
func (m *MockAnyNsClientServiceBase) GetNameByAddress(ctx context.Context, in *nameserviceproto.NameByAddressRequest) (*nameserviceproto.NameByAddressResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNameByAddress", ctx, in)
	ret0, _ := ret[0].(*nameserviceproto.NameByAddressResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNameByAddress indicates an expected call of GetNameByAddress.
func (mr *MockAnyNsClientServiceBaseMockRecorder) GetNameByAddress(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNameByAddress", reflect.TypeOf((*MockAnyNsClientServiceBase)(nil).GetNameByAddress), ctx, in)
}

// Init mocks base method.
func (m *MockAnyNsClientServiceBase) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockAnyNsClientServiceBaseMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockAnyNsClientServiceBase)(nil).Init), a)
}

// IsNameAvailable mocks base method.
func (m *MockAnyNsClientServiceBase) IsNameAvailable(ctx context.Context, in *nameserviceproto.NameAvailableRequest) (*nameserviceproto.NameAvailableResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsNameAvailable", ctx, in)
	ret0, _ := ret[0].(*nameserviceproto.NameAvailableResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsNameAvailable indicates an expected call of IsNameAvailable.
func (mr *MockAnyNsClientServiceBaseMockRecorder) IsNameAvailable(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsNameAvailable", reflect.TypeOf((*MockAnyNsClientServiceBase)(nil).IsNameAvailable), ctx, in)
}

// Name mocks base method.
func (m *MockAnyNsClientServiceBase) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockAnyNsClientServiceBaseMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockAnyNsClientServiceBase)(nil).Name))
}

// Run mocks base method.
func (m *MockAnyNsClientServiceBase) Run(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockAnyNsClientServiceBaseMockRecorder) Run(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockAnyNsClientServiceBase)(nil).Run), ctx)
}

// MockAnyNsClientService is a mock of AnyNsClientService interface.
type MockAnyNsClientService struct {
	ctrl     *gomock.Controller
	recorder *MockAnyNsClientServiceMockRecorder
}

// MockAnyNsClientServiceMockRecorder is the mock recorder for MockAnyNsClientService.
type MockAnyNsClientServiceMockRecorder struct {
	mock *MockAnyNsClientService
}

// NewMockAnyNsClientService creates a new mock instance.
func NewMockAnyNsClientService(ctrl *gomock.Controller) *MockAnyNsClientService {
	mock := &MockAnyNsClientService{ctrl: ctrl}
	mock.recorder = &MockAnyNsClientServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAnyNsClientService) EXPECT() *MockAnyNsClientServiceMockRecorder {
	return m.recorder
}

// AdminFundUserAccount mocks base method.
func (m *MockAnyNsClientService) AdminFundUserAccount(ctx context.Context, in *nameserviceproto.AdminFundUserAccountRequestSigned) (*nameserviceproto.OperationResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AdminFundUserAccount", ctx, in)
	ret0, _ := ret[0].(*nameserviceproto.OperationResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AdminFundUserAccount indicates an expected call of AdminFundUserAccount.
func (mr *MockAnyNsClientServiceMockRecorder) AdminFundUserAccount(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AdminFundUserAccount", reflect.TypeOf((*MockAnyNsClientService)(nil).AdminFundUserAccount), ctx, in)
}

// Close mocks base method.
func (m *MockAnyNsClientService) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockAnyNsClientServiceMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockAnyNsClientService)(nil).Close), ctx)
}

// CreateOperation mocks base method.
func (m *MockAnyNsClientService) CreateOperation(ctx context.Context, in *nameserviceproto.CreateUserOperationRequestSigned) (*nameserviceproto.OperationResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOperation", ctx, in)
	ret0, _ := ret[0].(*nameserviceproto.OperationResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateOperation indicates an expected call of CreateOperation.
func (mr *MockAnyNsClientServiceMockRecorder) CreateOperation(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOperation", reflect.TypeOf((*MockAnyNsClientService)(nil).CreateOperation), ctx, in)
}

// GetNameByAddress mocks base method.
func (m *MockAnyNsClientService) GetNameByAddress(ctx context.Context, in *nameserviceproto.NameByAddressRequest) (*nameserviceproto.NameByAddressResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNameByAddress", ctx, in)
	ret0, _ := ret[0].(*nameserviceproto.NameByAddressResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNameByAddress indicates an expected call of GetNameByAddress.
func (mr *MockAnyNsClientServiceMockRecorder) GetNameByAddress(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNameByAddress", reflect.TypeOf((*MockAnyNsClientService)(nil).GetNameByAddress), ctx, in)
}

// GetOperation mocks base method.
func (m *MockAnyNsClientService) GetOperation(ctx context.Context, in *nameserviceproto.GetOperationStatusRequest) (*nameserviceproto.OperationResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOperation", ctx, in)
	ret0, _ := ret[0].(*nameserviceproto.OperationResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOperation indicates an expected call of GetOperation.
func (mr *MockAnyNsClientServiceMockRecorder) GetOperation(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOperation", reflect.TypeOf((*MockAnyNsClientService)(nil).GetOperation), ctx, in)
}

// GetUserAccount mocks base method.
func (m *MockAnyNsClientService) GetUserAccount(ctx context.Context, in *nameserviceproto.GetUserAccountRequest) (*nameserviceproto.UserAccount, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserAccount", ctx, in)
	ret0, _ := ret[0].(*nameserviceproto.UserAccount)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserAccount indicates an expected call of GetUserAccount.
func (mr *MockAnyNsClientServiceMockRecorder) GetUserAccount(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserAccount", reflect.TypeOf((*MockAnyNsClientService)(nil).GetUserAccount), ctx, in)
}

// Init mocks base method.
func (m *MockAnyNsClientService) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockAnyNsClientServiceMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockAnyNsClientService)(nil).Init), a)
}

// IsNameAvailable mocks base method.
func (m *MockAnyNsClientService) IsNameAvailable(ctx context.Context, in *nameserviceproto.NameAvailableRequest) (*nameserviceproto.NameAvailableResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsNameAvailable", ctx, in)
	ret0, _ := ret[0].(*nameserviceproto.NameAvailableResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsNameAvailable indicates an expected call of IsNameAvailable.
func (mr *MockAnyNsClientServiceMockRecorder) IsNameAvailable(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsNameAvailable", reflect.TypeOf((*MockAnyNsClientService)(nil).IsNameAvailable), ctx, in)
}

// Name mocks base method.
func (m *MockAnyNsClientService) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockAnyNsClientServiceMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockAnyNsClientService)(nil).Name))
}

// Run mocks base method.
func (m *MockAnyNsClientService) Run(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockAnyNsClientServiceMockRecorder) Run(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockAnyNsClientService)(nil).Run), ctx)
}