// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/commonspace/headsync (interfaces: DiffSyncer)
//
// Generated by this command:
//
//	mockgen -destination mock_headsync/mock_headsync.go github.com/anyproto/any-sync/commonspace/headsync DiffSyncer
//

// Package mock_headsync is a generated GoMock package.
package mock_headsync

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockDiffSyncer is a mock of DiffSyncer interface.
type MockDiffSyncer struct {
	ctrl     *gomock.Controller
	recorder *MockDiffSyncerMockRecorder
	isgomock struct{}
}

// MockDiffSyncerMockRecorder is the mock recorder for MockDiffSyncer.
type MockDiffSyncerMockRecorder struct {
	mock *MockDiffSyncer
}

// NewMockDiffSyncer creates a new mock instance.
func NewMockDiffSyncer(ctrl *gomock.Controller) *MockDiffSyncer {
	mock := &MockDiffSyncer{ctrl: ctrl}
	mock.recorder = &MockDiffSyncerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDiffSyncer) EXPECT() *MockDiffSyncerMockRecorder {
	return m.recorder
}

// Init mocks base method.
func (m *MockDiffSyncer) Init() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Init")
}

// Init indicates an expected call of Init.
func (mr *MockDiffSyncerMockRecorder) Init() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockDiffSyncer)(nil).Init))
}

// RemoveObjects mocks base method.
func (m *MockDiffSyncer) RemoveObjects(ids []string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RemoveObjects", ids)
}

// RemoveObjects indicates an expected call of RemoveObjects.
func (mr *MockDiffSyncerMockRecorder) RemoveObjects(ids any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveObjects", reflect.TypeOf((*MockDiffSyncer)(nil).RemoveObjects), ids)
}

// Sync mocks base method.
func (m *MockDiffSyncer) Sync(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Sync", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Sync indicates an expected call of Sync.
func (mr *MockDiffSyncerMockRecorder) Sync(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sync", reflect.TypeOf((*MockDiffSyncer)(nil).Sync), ctx)
}

// UpdateHeads mocks base method.
func (m *MockDiffSyncer) UpdateHeads(id string, heads []string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateHeads", id, heads)
}

// UpdateHeads indicates an expected call of UpdateHeads.
func (mr *MockDiffSyncerMockRecorder) UpdateHeads(id, heads any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateHeads", reflect.TypeOf((*MockDiffSyncer)(nil).UpdateHeads), id, heads)
}
