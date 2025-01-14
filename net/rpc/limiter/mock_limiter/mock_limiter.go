// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/net/rpc/limiter (interfaces: RpcLimiter)
//
// Generated by this command:
//
//	mockgen -destination mock_limiter/mock_limiter.go github.com/anyproto/any-sync/net/rpc/limiter RpcLimiter
//

// Package mock_limiter is a generated GoMock package.
package mock_limiter

import (
	context "context"
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	gomock "go.uber.org/mock/gomock"
	drpc "storj.io/drpc"
)

// MockRpcLimiter is a mock of RpcLimiter interface.
type MockRpcLimiter struct {
	ctrl     *gomock.Controller
	recorder *MockRpcLimiterMockRecorder
	isgomock struct{}
}

// MockRpcLimiterMockRecorder is the mock recorder for MockRpcLimiter.
type MockRpcLimiterMockRecorder struct {
	mock *MockRpcLimiter
}

// NewMockRpcLimiter creates a new mock instance.
func NewMockRpcLimiter(ctrl *gomock.Controller) *MockRpcLimiter {
	mock := &MockRpcLimiter{ctrl: ctrl}
	mock.recorder = &MockRpcLimiterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRpcLimiter) EXPECT() *MockRpcLimiterMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockRpcLimiter) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockRpcLimiterMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockRpcLimiter)(nil).Close), ctx)
}

// Init mocks base method.
func (m *MockRpcLimiter) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockRpcLimiterMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockRpcLimiter)(nil).Init), a)
}

// Name mocks base method.
func (m *MockRpcLimiter) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockRpcLimiterMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockRpcLimiter)(nil).Name))
}

// Run mocks base method.
func (m *MockRpcLimiter) Run(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockRpcLimiterMockRecorder) Run(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockRpcLimiter)(nil).Run), ctx)
}

// WrapDRPCHandler mocks base method.
func (m *MockRpcLimiter) WrapDRPCHandler(handler drpc.Handler) drpc.Handler {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WrapDRPCHandler", handler)
	ret0, _ := ret[0].(drpc.Handler)
	return ret0
}

// WrapDRPCHandler indicates an expected call of WrapDRPCHandler.
func (mr *MockRpcLimiterMockRecorder) WrapDRPCHandler(handler any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WrapDRPCHandler", reflect.TypeOf((*MockRpcLimiter)(nil).WrapDRPCHandler), handler)
}
