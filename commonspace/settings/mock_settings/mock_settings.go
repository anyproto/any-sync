// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anytypeio/any-sync/commonspace/settings (interfaces: DeletionManager,Deleter,SpaceIdsProvider)

// Package mock_settings is a generated GoMock package.
package mock_settings

import (
	context "context"
	reflect "reflect"

	settingsstate "github.com/anytypeio/any-sync/commonspace/settings/settingsstate"
	gomock "github.com/golang/mock/gomock"
)

// MockDeletionManager is a mock of DeletionManager interface.
type MockDeletionManager struct {
	ctrl     *gomock.Controller
	recorder *MockDeletionManagerMockRecorder
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

// UpdateState mocks base method.
func (m *MockDeletionManager) UpdateState(arg0 context.Context, arg1 *settingsstate.State) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateState", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateState indicates an expected call of UpdateState.
func (mr *MockDeletionManagerMockRecorder) UpdateState(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateState", reflect.TypeOf((*MockDeletionManager)(nil).UpdateState), arg0, arg1)
}

// MockDeleter is a mock of Deleter interface.
type MockDeleter struct {
	ctrl     *gomock.Controller
	recorder *MockDeleterMockRecorder
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
func (m *MockDeleter) Delete() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Delete")
}

// Delete indicates an expected call of Delete.
func (mr *MockDeleterMockRecorder) Delete() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockDeleter)(nil).Delete))
}

// MockSpaceIdsProvider is a mock of SpaceIdsProvider interface.
type MockSpaceIdsProvider struct {
	ctrl     *gomock.Controller
	recorder *MockSpaceIdsProviderMockRecorder
}

// MockSpaceIdsProviderMockRecorder is the mock recorder for MockSpaceIdsProvider.
type MockSpaceIdsProviderMockRecorder struct {
	mock *MockSpaceIdsProvider
}

// NewMockSpaceIdsProvider creates a new mock instance.
func NewMockSpaceIdsProvider(ctrl *gomock.Controller) *MockSpaceIdsProvider {
	mock := &MockSpaceIdsProvider{ctrl: ctrl}
	mock.recorder = &MockSpaceIdsProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSpaceIdsProvider) EXPECT() *MockSpaceIdsProviderMockRecorder {
	return m.recorder
}

// AllIds mocks base method.
func (m *MockSpaceIdsProvider) AllIds() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AllIds")
	ret0, _ := ret[0].([]string)
	return ret0
}

// AllIds indicates an expected call of AllIds.
func (mr *MockSpaceIdsProviderMockRecorder) AllIds() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AllIds", reflect.TypeOf((*MockSpaceIdsProvider)(nil).AllIds))
}
