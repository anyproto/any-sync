// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/commonspace/headsync/statestorage (interfaces: StateStorage)
//
// Generated by this command:
//
//	mockgen -destination mock_statestorage/mock_statestorage.go github.com/anyproto/any-sync/commonspace/headsync/statestorage StateStorage
//
// Package mock_statestorage is a generated GoMock package.
package mock_statestorage

import (
	context "context"
	reflect "reflect"

	statestorage "github.com/anyproto/any-sync/commonspace/headsync/statestorage"
	gomock "go.uber.org/mock/gomock"
)

// MockStateStorage is a mock of StateStorage interface.
type MockStateStorage struct {
	ctrl     *gomock.Controller
	recorder *MockStateStorageMockRecorder
}

// MockStateStorageMockRecorder is the mock recorder for MockStateStorage.
type MockStateStorageMockRecorder struct {
	mock *MockStateStorage
}

// NewMockStateStorage creates a new mock instance.
func NewMockStateStorage(ctrl *gomock.Controller) *MockStateStorage {
	mock := &MockStateStorage{ctrl: ctrl}
	mock.recorder = &MockStateStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStateStorage) EXPECT() *MockStateStorageMockRecorder {
	return m.recorder
}

// GetState mocks base method.
func (m *MockStateStorage) GetState(arg0 context.Context) (statestorage.State, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetState", arg0)
	ret0, _ := ret[0].(statestorage.State)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetState indicates an expected call of GetState.
func (mr *MockStateStorageMockRecorder) GetState(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetState", reflect.TypeOf((*MockStateStorage)(nil).GetState), arg0)
}

// SetHash mocks base method.
func (m *MockStateStorage) SetHash(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetHash", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetHash indicates an expected call of SetHash.
func (mr *MockStateStorageMockRecorder) SetHash(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetHash", reflect.TypeOf((*MockStateStorage)(nil).SetHash), arg0, arg1)
}

// SetObserver mocks base method.
func (m *MockStateStorage) SetObserver(arg0 statestorage.Observer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetObserver", arg0)
}

// SetObserver indicates an expected call of SetObserver.
func (mr *MockStateStorageMockRecorder) SetObserver(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetObserver", reflect.TypeOf((*MockStateStorage)(nil).SetObserver), arg0)
}

// SettingsId mocks base method.
func (m *MockStateStorage) SettingsId() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SettingsId")
	ret0, _ := ret[0].(string)
	return ret0
}

// SettingsId indicates an expected call of SettingsId.
func (mr *MockStateStorageMockRecorder) SettingsId() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SettingsId", reflect.TypeOf((*MockStateStorage)(nil).SettingsId))
}
