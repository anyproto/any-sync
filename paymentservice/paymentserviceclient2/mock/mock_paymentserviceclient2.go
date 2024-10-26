// Code generated by MockGen. DO NOT EDIT.
// Source: paymentservice/paymentserviceclient2/paymentserviceclient2.go
//
// Generated by this command:
//
//	mockgen -source paymentservice/paymentserviceclient2/paymentserviceclient2.go
//

// Package mock_paymentserviceclient2 is a generated GoMock package.
package mock_paymentserviceclient2

import (
	context "context"
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	paymentserviceproto "github.com/anyproto/any-sync/paymentservice/paymentserviceproto"
	gomock "go.uber.org/mock/gomock"
)

// MockAnyPpClientService2 is a mock of AnyPpClientService2 interface.
type MockAnyPpClientService2 struct {
	ctrl     *gomock.Controller
	recorder *MockAnyPpClientService2MockRecorder
}

// MockAnyPpClientService2MockRecorder is the mock recorder for MockAnyPpClientService2.
type MockAnyPpClientService2MockRecorder struct {
	mock *MockAnyPpClientService2
}

// NewMockAnyPpClientService2 creates a new mock instance.
func NewMockAnyPpClientService2(ctrl *gomock.Controller) *MockAnyPpClientService2 {
	mock := &MockAnyPpClientService2{ctrl: ctrl}
	mock.recorder = &MockAnyPpClientService2MockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAnyPpClientService2) EXPECT() *MockAnyPpClientService2MockRecorder {
	return m.recorder
}

// GetStatus mocks base method.
func (m *MockAnyPpClientService2) GetStatus(ctx context.Context, in *paymentserviceproto.Membership2_GetStatusRequest) (*paymentserviceproto.Membership2_GetStatusResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStatus", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.Membership2_GetStatusResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetStatus indicates an expected call of GetStatus.
func (mr *MockAnyPpClientService2MockRecorder) GetStatus(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStatus", reflect.TypeOf((*MockAnyPpClientService2)(nil).GetStatus), ctx, in)
}

// Init mocks base method.
func (m *MockAnyPpClientService2) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockAnyPpClientService2MockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockAnyPpClientService2)(nil).Init), a)
}

// Name mocks base method.
func (m *MockAnyPpClientService2) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockAnyPpClientService2MockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockAnyPpClientService2)(nil).Name))
}

// ProductAllocateToSpace mocks base method.
func (m *MockAnyPpClientService2) ProductAllocateToSpace(ctx context.Context, in *paymentserviceproto.Membership2_ProductAllocateToSpaceRequest) (*paymentserviceproto.Membership2_ProductAllocateToSpaceResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ProductAllocateToSpace", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.Membership2_ProductAllocateToSpaceResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ProductAllocateToSpace indicates an expected call of ProductAllocateToSpace.
func (mr *MockAnyPpClientService2MockRecorder) ProductAllocateToSpace(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ProductAllocateToSpace", reflect.TypeOf((*MockAnyPpClientService2)(nil).ProductAllocateToSpace), ctx, in)
}

// StoreCartCheckoutGenerate mocks base method.
func (m *MockAnyPpClientService2) StoreCartCheckoutGenerate(ctx context.Context, in *paymentserviceproto.Membership2_StoreCartCheckoutGenerateRequest) (*paymentserviceproto.Membership2_StoreCartCheckoutGenerateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreCartCheckoutGenerate", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.Membership2_StoreCartCheckoutGenerateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StoreCartCheckoutGenerate indicates an expected call of StoreCartCheckoutGenerate.
func (mr *MockAnyPpClientService2MockRecorder) StoreCartCheckoutGenerate(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreCartCheckoutGenerate", reflect.TypeOf((*MockAnyPpClientService2)(nil).StoreCartCheckoutGenerate), ctx, in)
}

// StoreCartGet mocks base method.
func (m *MockAnyPpClientService2) StoreCartGet(ctx context.Context, in *paymentserviceproto.Membership2_StoreCartGetRequest) (*paymentserviceproto.Membership2_StoreCartGetResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreCartGet", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.Membership2_StoreCartGetResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StoreCartGet indicates an expected call of StoreCartGet.
func (mr *MockAnyPpClientService2MockRecorder) StoreCartGet(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreCartGet", reflect.TypeOf((*MockAnyPpClientService2)(nil).StoreCartGet), ctx, in)
}

// StoreCartProductAdd mocks base method.
func (m *MockAnyPpClientService2) StoreCartProductAdd(ctx context.Context, in *paymentserviceproto.Membership2_StoreCartProductAddRequest) (*paymentserviceproto.Membership2_StoreCartProductAddResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreCartProductAdd", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.Membership2_StoreCartProductAddResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StoreCartProductAdd indicates an expected call of StoreCartProductAdd.
func (mr *MockAnyPpClientService2MockRecorder) StoreCartProductAdd(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreCartProductAdd", reflect.TypeOf((*MockAnyPpClientService2)(nil).StoreCartProductAdd), ctx, in)
}

// StoreCartProductRemove mocks base method.
func (m *MockAnyPpClientService2) StoreCartProductRemove(ctx context.Context, in *paymentserviceproto.Membership2_StoreCartProductRemoveRequest) (*paymentserviceproto.Membership2_StoreCartProductRemoveResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreCartProductRemove", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.Membership2_StoreCartProductRemoveResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StoreCartProductRemove indicates an expected call of StoreCartProductRemove.
func (mr *MockAnyPpClientService2MockRecorder) StoreCartProductRemove(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreCartProductRemove", reflect.TypeOf((*MockAnyPpClientService2)(nil).StoreCartProductRemove), ctx, in)
}

// StoreCartPromocodeApply mocks base method.
func (m *MockAnyPpClientService2) StoreCartPromocodeApply(ctx context.Context, in *paymentserviceproto.Membership2_StoreCartPromocodeApplyRequest) (*paymentserviceproto.Membership2_StoreCartPromocodeApplyResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreCartPromocodeApply", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.Membership2_StoreCartPromocodeApplyResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StoreCartPromocodeApply indicates an expected call of StoreCartPromocodeApply.
func (mr *MockAnyPpClientService2MockRecorder) StoreCartPromocodeApply(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreCartPromocodeApply", reflect.TypeOf((*MockAnyPpClientService2)(nil).StoreCartPromocodeApply), ctx, in)
}

// StoreProductsEnumerate mocks base method.
func (m *MockAnyPpClientService2) StoreProductsEnumerate(ctx context.Context, in *paymentserviceproto.Membership2_StoreProductsEnumerateRequest) (*paymentserviceproto.Membership2_StoreProductsEnumerateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreProductsEnumerate", ctx, in)
	ret0, _ := ret[0].(*paymentserviceproto.Membership2_StoreProductsEnumerateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StoreProductsEnumerate indicates an expected call of StoreProductsEnumerate.
func (mr *MockAnyPpClientService2MockRecorder) StoreProductsEnumerate(ctx, in any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreProductsEnumerate", reflect.TypeOf((*MockAnyPpClientService2)(nil).StoreProductsEnumerate), ctx, in)
}
