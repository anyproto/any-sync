// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/commonspace/deletionstate (interfaces: ObjectDeletionState)
//
// Generated by this command:
//
//	mockgen -destination mock_deletionstate/mock_deletionstate.go github.com/anyproto/any-sync/commonspace/deletionstate ObjectDeletionState
//

// Package mock_deletionstate is a generated GoMock package.
package mock_deletionstate

import (
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	deletionstate "github.com/anyproto/any-sync/commonspace/deletionstate"
	gomock "go.uber.org/mock/gomock"
)

// MockObjectDeletionState is a mock of ObjectDeletionState interface.
type MockObjectDeletionState struct {
	ctrl     *gomock.Controller
	recorder *MockObjectDeletionStateMockRecorder
	isgomock struct{}
}

// MockObjectDeletionStateMockRecorder is the mock recorder for MockObjectDeletionState.
type MockObjectDeletionStateMockRecorder struct {
	mock *MockObjectDeletionState
}

// NewMockObjectDeletionState creates a new mock instance.
func NewMockObjectDeletionState(ctrl *gomock.Controller) *MockObjectDeletionState {
	mock := &MockObjectDeletionState{ctrl: ctrl}
	mock.recorder = &MockObjectDeletionStateMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockObjectDeletionState) EXPECT() *MockObjectDeletionStateMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockObjectDeletionState) Add(ids map[string]struct{}) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Add", ids)
}

// Add indicates an expected call of Add.
func (mr *MockObjectDeletionStateMockRecorder) Add(ids any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockObjectDeletionState)(nil).Add), ids)
}

// AddObserver mocks base method.
func (m *MockObjectDeletionState) AddObserver(observer deletionstate.StateUpdateObserver) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddObserver", observer)
}

// AddObserver indicates an expected call of AddObserver.
func (mr *MockObjectDeletionStateMockRecorder) AddObserver(observer any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddObserver", reflect.TypeOf((*MockObjectDeletionState)(nil).AddObserver), observer)
}

// Delete mocks base method.
func (m *MockObjectDeletionState) Delete(id string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", id)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockObjectDeletionStateMockRecorder) Delete(id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockObjectDeletionState)(nil).Delete), id)
}

// Exists mocks base method.
func (m *MockObjectDeletionState) Exists(id string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Exists", id)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Exists indicates an expected call of Exists.
func (mr *MockObjectDeletionStateMockRecorder) Exists(id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exists", reflect.TypeOf((*MockObjectDeletionState)(nil).Exists), id)
}

// Filter mocks base method.
func (m *MockObjectDeletionState) Filter(ids []string) []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Filter", ids)
	ret0, _ := ret[0].([]string)
	return ret0
}

// Filter indicates an expected call of Filter.
func (mr *MockObjectDeletionStateMockRecorder) Filter(ids any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Filter", reflect.TypeOf((*MockObjectDeletionState)(nil).Filter), ids)
}

// GetQueued mocks base method.
func (m *MockObjectDeletionState) GetQueued() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetQueued")
	ret0, _ := ret[0].([]string)
	return ret0
}

// GetQueued indicates an expected call of GetQueued.
func (mr *MockObjectDeletionStateMockRecorder) GetQueued() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetQueued", reflect.TypeOf((*MockObjectDeletionState)(nil).GetQueued))
}

// Init mocks base method.
func (m *MockObjectDeletionState) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockObjectDeletionStateMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockObjectDeletionState)(nil).Init), a)
}

// Name mocks base method.
func (m *MockObjectDeletionState) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockObjectDeletionStateMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockObjectDeletionState)(nil).Name))
}
