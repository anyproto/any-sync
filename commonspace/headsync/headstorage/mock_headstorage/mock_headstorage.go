// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/commonspace/headsync/headstorage (interfaces: HeadStorage)
//
// Generated by this command:
//
//	mockgen -destination mock_headstorage/mock_headstorage.go github.com/anyproto/any-sync/commonspace/headsync/headstorage HeadStorage
//
// Package mock_headstorage is a generated GoMock package.
package mock_headstorage

import (
	reflect "reflect"

	headstorage "github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	gomock "go.uber.org/mock/gomock"
	context "golang.org/x/net/context"
)

// MockHeadStorage is a mock of HeadStorage interface.
type MockHeadStorage struct {
	ctrl     *gomock.Controller
	recorder *MockHeadStorageMockRecorder
	isgomock struct{}
}

// MockHeadStorageMockRecorder is the mock recorder for MockHeadStorage.
type MockHeadStorageMockRecorder struct {
	mock *MockHeadStorage
}

// NewMockHeadStorage creates a new mock instance.
func NewMockHeadStorage(ctrl *gomock.Controller) *MockHeadStorage {
	mock := &MockHeadStorage{ctrl: ctrl}
	mock.recorder = &MockHeadStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHeadStorage) EXPECT() *MockHeadStorageMockRecorder {
	return m.recorder
}

// AddObserver mocks base method.
func (m *MockHeadStorage) AddObserver(observer headstorage.Observer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddObserver", observer)
}

// AddObserver indicates an expected call of AddObserver.
func (mr *MockHeadStorageMockRecorder) AddObserver(observer any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddObserver", reflect.TypeOf((*MockHeadStorage)(nil).AddObserver), observer)
}

// DeleteEntryTx mocks base method.
func (m *MockHeadStorage) DeleteEntryTx(txCtx context.Context, id string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteEntryTx", txCtx, id)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteEntryTx indicates an expected call of DeleteEntryTx.
func (mr *MockHeadStorageMockRecorder) DeleteEntryTx(txCtx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteEntryTx", reflect.TypeOf((*MockHeadStorage)(nil).DeleteEntryTx), txCtx, id)
}

// GetEntry mocks base method.
func (m *MockHeadStorage) GetEntry(ctx context.Context, id string) (headstorage.HeadsEntry, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEntry", ctx, id)
	ret0, _ := ret[0].(headstorage.HeadsEntry)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetEntry indicates an expected call of GetEntry.
func (mr *MockHeadStorageMockRecorder) GetEntry(ctx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEntry", reflect.TypeOf((*MockHeadStorage)(nil).GetEntry), ctx, id)
}

// IterateEntries mocks base method.
func (m *MockHeadStorage) IterateEntries(ctx context.Context, iterOpts headstorage.IterOpts, iter headstorage.EntryIterator) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IterateEntries", ctx, iterOpts, iter)
	ret0, _ := ret[0].(error)
	return ret0
}

// IterateEntries indicates an expected call of IterateEntries.
func (mr *MockHeadStorageMockRecorder) IterateEntries(ctx, iterOpts, iter any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IterateEntries", reflect.TypeOf((*MockHeadStorage)(nil).IterateEntries), ctx, iterOpts, iter)
}

// UpdateEntry mocks base method.
func (m *MockHeadStorage) UpdateEntry(ctx context.Context, update headstorage.HeadsUpdate) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateEntry", ctx, update)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateEntry indicates an expected call of UpdateEntry.
func (mr *MockHeadStorageMockRecorder) UpdateEntry(ctx, update any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateEntry", reflect.TypeOf((*MockHeadStorage)(nil).UpdateEntry), ctx, update)
}

// UpdateEntryTx mocks base method.
func (m *MockHeadStorage) UpdateEntryTx(txCtx context.Context, update headstorage.HeadsUpdate) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateEntryTx", txCtx, update)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateEntryTx indicates an expected call of UpdateEntryTx.
func (mr *MockHeadStorageMockRecorder) UpdateEntryTx(txCtx, update any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateEntryTx", reflect.TypeOf((*MockHeadStorage)(nil).UpdateEntryTx), txCtx, update)
}
