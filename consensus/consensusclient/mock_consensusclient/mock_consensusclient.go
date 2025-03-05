// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/consensus/consensusclient (interfaces: Service)
//
// Generated by this command:
//
//	mockgen -destination mock_consensusclient/mock_consensusclient.go github.com/anyproto/any-sync/consensus/consensusclient Service
//
// Package mock_consensusclient is a generated GoMock package.
package mock_consensusclient

import (
	context "context"
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	consensusclient "github.com/anyproto/any-sync/consensus/consensusclient"
	consensusproto "github.com/anyproto/any-sync/consensus/consensusproto"
	gomock "go.uber.org/mock/gomock"
)

// MockService is a mock of Service interface.
type MockService struct {
	ctrl     *gomock.Controller
	recorder *MockServiceMockRecorder
	isgomock struct{}
}

// MockServiceMockRecorder is the mock recorder for MockService.
type MockServiceMockRecorder struct {
	mock *MockService
}

// NewMockService creates a new mock instance.
func NewMockService(ctrl *gomock.Controller) *MockService {
	mock := &MockService{ctrl: ctrl}
	mock.recorder = &MockServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockService) EXPECT() *MockServiceMockRecorder {
	return m.recorder
}

// AddLog mocks base method.
func (m *MockService) AddLog(ctx context.Context, logId string, rec *consensusproto.RawRecordWithId) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddLog", ctx, logId, rec)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddLog indicates an expected call of AddLog.
func (mr *MockServiceMockRecorder) AddLog(ctx, logId, rec any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddLog", reflect.TypeOf((*MockService)(nil).AddLog), ctx, logId, rec)
}

// AddRecord mocks base method.
func (m *MockService) AddRecord(ctx context.Context, logId string, rec *consensusproto.RawRecord) (*consensusproto.RawRecordWithId, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddRecord", ctx, logId, rec)
	ret0, _ := ret[0].(*consensusproto.RawRecordWithId)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddRecord indicates an expected call of AddRecord.
func (mr *MockServiceMockRecorder) AddRecord(ctx, logId, rec any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRecord", reflect.TypeOf((*MockService)(nil).AddRecord), ctx, logId, rec)
}

// Close mocks base method.
func (m *MockService) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockServiceMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockService)(nil).Close), ctx)
}

// DeleteLog mocks base method.
func (m *MockService) DeleteLog(ctx context.Context, logId string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteLog", ctx, logId)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteLog indicates an expected call of DeleteLog.
func (mr *MockServiceMockRecorder) DeleteLog(ctx, logId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteLog", reflect.TypeOf((*MockService)(nil).DeleteLog), ctx, logId)
}

// Init mocks base method.
func (m *MockService) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockServiceMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockService)(nil).Init), a)
}

// Name mocks base method.
func (m *MockService) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockServiceMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockService)(nil).Name))
}

// Run mocks base method.
func (m *MockService) Run(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockServiceMockRecorder) Run(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockService)(nil).Run), ctx)
}

// UnWatch mocks base method.
func (m *MockService) UnWatch(logId string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnWatch", logId)
	ret0, _ := ret[0].(error)
	return ret0
}

// UnWatch indicates an expected call of UnWatch.
func (mr *MockServiceMockRecorder) UnWatch(logId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnWatch", reflect.TypeOf((*MockService)(nil).UnWatch), logId)
}

// Watch mocks base method.
func (m *MockService) Watch(logId string, w consensusclient.Watcher) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Watch", logId, w)
	ret0, _ := ret[0].(error)
	return ret0
}

// Watch indicates an expected call of Watch.
func (mr *MockServiceMockRecorder) Watch(logId, w any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Watch", reflect.TypeOf((*MockService)(nil).Watch), logId, w)
}
