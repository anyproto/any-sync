// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/commonspace/deletionmanager (interfaces: DeletionManager,Deleter)
//
// Generated by this command:
//
//	mockgen -destination mock_deletionmanager/mock_deletionmanager.go github.com/anyproto/any-sync/commonspace/deletionmanager DeletionManager,Deleter
//
// Package mock_deletionmanager is a generated GoMock package.
package mock_deletionmanager

import (
	context "context"
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	settingsstate "github.com/anyproto/any-sync/commonspace/settings/settingsstate"
	gomock "go.uber.org/mock/gomock"
)

// MockDeletionManager is a mock of DeletionManager interface.
type MockDeletionManager struct {
	ctrl     *gomock.Controller
	recorder *MockDeletionManagerMockRecorder
	isgomock struct{}
}

// MockDeletionManagerMockRecorder is the mock recorder for MockDeletionManager.
type MockDeletionManagerMockRecorder struct {
	mock *MockDeletionManager
}

// NewMockDeletionManager creates a new mock instance.
func NewMockDeletionManager(ctrl *gomock.Controller) *MockDeletionManager {
	mock := &MockDeletionManager{ctrl: ctrl}
	mock.recorder = &MockDeletionManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDeletionManager) EXPECT() *MockDeletionManagerMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockDeletionManager) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockDeletionManagerMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockDeletionManager)(nil).Close), ctx)
}

// Init mocks base method.
func (m *MockDeletionManager) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockDeletionManagerMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockDeletionManager)(nil).Init), a)
}

// Name mocks base method.
func (m *MockDeletionManager) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockDeletionManagerMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockDeletionManager)(nil).Name))
}

// Run mocks base method.
func (m *MockDeletionManager) Run(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockDeletionManagerMockRecorder) Run(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockDeletionManager)(nil).Run), ctx)
}

// UpdateState mocks base method.
func (m *MockDeletionManager) UpdateState(ctx context.Context, state *settingsstate.State) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateState", ctx, state)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateState indicates an expected call of UpdateState.
func (mr *MockDeletionManagerMockRecorder) UpdateState(ctx, state any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateState", reflect.TypeOf((*MockDeletionManager)(nil).UpdateState), ctx, state)
}

// MockDeleter is a mock of Deleter interface.
type MockDeleter struct {
	ctrl     *gomock.Controller
	recorder *MockDeleterMockRecorder
	isgomock struct{}
}

// MockDeleterMockRecorder is the mock recorder for MockDeleter.
type MockDeleterMockRecorder struct {
	mock *MockDeleter
}

// NewMockDeleter creates a new mock instance.
func NewMockDeleter(ctrl *gomock.Controller) *MockDeleter {
	mock := &MockDeleter{ctrl: ctrl}
	mock.recorder = &MockDeleterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDeleter) EXPECT() *MockDeleterMockRecorder {
	return m.recorder
}

// Delete mocks base method.
func (m *MockDeleter) Delete(ctx context.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Delete", ctx)
}

// Delete indicates an expected call of Delete.
func (mr *MockDeleterMockRecorder) Delete(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockDeleter)(nil).Delete), ctx)
}
