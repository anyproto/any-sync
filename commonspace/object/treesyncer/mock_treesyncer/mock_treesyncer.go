// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/commonspace/object/treesyncer (interfaces: TreeSyncer)
//
// Generated by this command:
//
//	mockgen -destination mock_treesyncer/mock_treesyncer.go github.com/anyproto/any-sync/commonspace/object/treesyncer TreeSyncer
//
// Package mock_treesyncer is a generated GoMock package.
package mock_treesyncer

import (
	context "context"
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	gomock "go.uber.org/mock/gomock"
)

// MockTreeSyncer is a mock of TreeSyncer interface.
type MockTreeSyncer struct {
	ctrl     *gomock.Controller
	recorder *MockTreeSyncerMockRecorder
}

// MockTreeSyncerMockRecorder is the mock recorder for MockTreeSyncer.
type MockTreeSyncerMockRecorder struct {
	mock *MockTreeSyncer
}

// NewMockTreeSyncer creates a new mock instance.
func NewMockTreeSyncer(ctrl *gomock.Controller) *MockTreeSyncer {
	mock := &MockTreeSyncer{ctrl: ctrl}
	mock.recorder = &MockTreeSyncerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTreeSyncer) EXPECT() *MockTreeSyncerMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockTreeSyncer) Close(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockTreeSyncerMockRecorder) Close(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockTreeSyncer)(nil).Close), arg0)
}

// Init mocks base method.
func (m *MockTreeSyncer) Init(arg0 *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockTreeSyncerMockRecorder) Init(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockTreeSyncer)(nil).Init), arg0)
}

// Name mocks base method.
func (m *MockTreeSyncer) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockTreeSyncerMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockTreeSyncer)(nil).Name))
}

// Run mocks base method.
func (m *MockTreeSyncer) Run(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockTreeSyncerMockRecorder) Run(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockTreeSyncer)(nil).Run), arg0)
}

// StartSync mocks base method.
func (m *MockTreeSyncer) StartSync() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "StartSync")
}

// StartSync indicates an expected call of StartSync.
func (mr *MockTreeSyncerMockRecorder) StartSync() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartSync", reflect.TypeOf((*MockTreeSyncer)(nil).StartSync))
}

// SyncAll mocks base method.
func (m *MockTreeSyncer) SyncAll(arg0 context.Context, arg1 string, arg2, arg3 []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncAll", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// SyncAll indicates an expected call of SyncAll.
func (mr *MockTreeSyncerMockRecorder) SyncAll(arg0, arg1, arg2, arg3 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncAll", reflect.TypeOf((*MockTreeSyncer)(nil).SyncAll), arg0, arg1, arg2, arg3)
}