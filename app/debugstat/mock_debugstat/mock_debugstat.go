// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/app/debugstat (interfaces: StatService)
//
// Generated by this command:
//
//	mockgen -destination mock_debugstat/mock_debugstat.go github.com/anyproto/any-sync/app/debugstat StatService
//
// Package mock_debugstat is a generated GoMock package.
package mock_debugstat

import (
	context "context"
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	debugstat "github.com/anyproto/any-sync/app/debugstat"
	gomock "go.uber.org/mock/gomock"
)

// MockStatService is a mock of StatService interface.
type MockStatService struct {
	ctrl     *gomock.Controller
	recorder *MockStatServiceMockRecorder
	isgomock struct{}
}

// MockStatServiceMockRecorder is the mock recorder for MockStatService.
type MockStatServiceMockRecorder struct {
	mock *MockStatService
}

// NewMockStatService creates a new mock instance.
func NewMockStatService(ctrl *gomock.Controller) *MockStatService {
	mock := &MockStatService{ctrl: ctrl}
	mock.recorder = &MockStatServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStatService) EXPECT() *MockStatServiceMockRecorder {
	return m.recorder
}

// AddProvider mocks base method.
func (m *MockStatService) AddProvider(provider debugstat.StatProvider) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddProvider", provider)
}

// AddProvider indicates an expected call of AddProvider.
func (mr *MockStatServiceMockRecorder) AddProvider(provider any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddProvider", reflect.TypeOf((*MockStatService)(nil).AddProvider), provider)
}

// Close mocks base method.
func (m *MockStatService) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockStatServiceMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockStatService)(nil).Close), ctx)
}

// GetStat mocks base method.
func (m *MockStatService) GetStat() debugstat.StatSummary {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStat")
	ret0, _ := ret[0].(debugstat.StatSummary)
	return ret0
}

// GetStat indicates an expected call of GetStat.
func (mr *MockStatServiceMockRecorder) GetStat() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStat", reflect.TypeOf((*MockStatService)(nil).GetStat))
}

// Init mocks base method.
func (m *MockStatService) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockStatServiceMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockStatService)(nil).Init), a)
}

// Name mocks base method.
func (m *MockStatService) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockStatServiceMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockStatService)(nil).Name))
}

// RemoveProvider mocks base method.
func (m *MockStatService) RemoveProvider(provider debugstat.StatProvider) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RemoveProvider", provider)
}

// RemoveProvider indicates an expected call of RemoveProvider.
func (mr *MockStatServiceMockRecorder) RemoveProvider(provider any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveProvider", reflect.TypeOf((*MockStatService)(nil).RemoveProvider), provider)
}

// Run mocks base method.
func (m *MockStatService) Run(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockStatServiceMockRecorder) Run(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockStatService)(nil).Run), ctx)
}
