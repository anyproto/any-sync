// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/commonspace/spacestorage (interfaces: SpaceStorage)
//
// Generated by this command:
//
//	mockgen -destination mock_spacestorage/mock_spacestorage.go github.com/anyproto/any-sync/commonspace/spacestorage SpaceStorage
//
// Package mock_spacestorage is a generated GoMock package.
package mock_spacestorage

import (
	context "context"
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	liststorage "github.com/anyproto/any-sync/commonspace/object/acl/liststorage"
	treechangeproto "github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	treestorage "github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	spacesyncproto "github.com/anyproto/any-sync/commonspace/spacesyncproto"
	gomock "go.uber.org/mock/gomock"
)

// MockSpaceStorage is a mock of SpaceStorage interface.
type MockSpaceStorage struct {
	ctrl     *gomock.Controller
	recorder *MockSpaceStorageMockRecorder
}

// MockSpaceStorageMockRecorder is the mock recorder for MockSpaceStorage.
type MockSpaceStorageMockRecorder struct {
	mock *MockSpaceStorage
}

// NewMockSpaceStorage creates a new mock instance.
func NewMockSpaceStorage(ctrl *gomock.Controller) *MockSpaceStorage {
	mock := &MockSpaceStorage{ctrl: ctrl}
	mock.recorder = &MockSpaceStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSpaceStorage) EXPECT() *MockSpaceStorageMockRecorder {
	return m.recorder
}

// AclStorage mocks base method.
func (m *MockSpaceStorage) AclStorage() (liststorage.ListStorage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AclStorage")
	ret0, _ := ret[0].(liststorage.ListStorage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AclStorage indicates an expected call of AclStorage.
func (mr *MockSpaceStorageMockRecorder) AclStorage() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AclStorage", reflect.TypeOf((*MockSpaceStorage)(nil).AclStorage))
}

// Close mocks base method.
func (m *MockSpaceStorage) Close(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockSpaceStorageMockRecorder) Close(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockSpaceStorage)(nil).Close), arg0)
}

// CreateTreeStorage mocks base method.
func (m *MockSpaceStorage) CreateTreeStorage(arg0 treestorage.TreeStorageCreatePayload) (treestorage.TreeStorage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTreeStorage", arg0)
	ret0, _ := ret[0].(treestorage.TreeStorage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateTreeStorage indicates an expected call of CreateTreeStorage.
func (mr *MockSpaceStorageMockRecorder) CreateTreeStorage(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTreeStorage", reflect.TypeOf((*MockSpaceStorage)(nil).CreateTreeStorage), arg0)
}

// HasTree mocks base method.
func (m *MockSpaceStorage) HasTree(arg0 string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasTree", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HasTree indicates an expected call of HasTree.
func (mr *MockSpaceStorageMockRecorder) HasTree(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasTree", reflect.TypeOf((*MockSpaceStorage)(nil).HasTree), arg0)
}

// Id mocks base method.
func (m *MockSpaceStorage) Id() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Id")
	ret0, _ := ret[0].(string)
	return ret0
}

// Id indicates an expected call of Id.
func (mr *MockSpaceStorageMockRecorder) Id() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Id", reflect.TypeOf((*MockSpaceStorage)(nil).Id))
}

// Init mocks base method.
func (m *MockSpaceStorage) Init(arg0 *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockSpaceStorageMockRecorder) Init(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockSpaceStorage)(nil).Init), arg0)
}

// IsSpaceDeleted mocks base method.
func (m *MockSpaceStorage) IsSpaceDeleted() (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsSpaceDeleted")
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsSpaceDeleted indicates an expected call of IsSpaceDeleted.
func (mr *MockSpaceStorageMockRecorder) IsSpaceDeleted() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsSpaceDeleted", reflect.TypeOf((*MockSpaceStorage)(nil).IsSpaceDeleted))
}

// Name mocks base method.
func (m *MockSpaceStorage) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockSpaceStorageMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockSpaceStorage)(nil).Name))
}

// ReadOldSpaceHash mocks base method.
func (m *MockSpaceStorage) ReadOldSpaceHash() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadOldSpaceHash")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadOldSpaceHash indicates an expected call of ReadOldSpaceHash.
func (mr *MockSpaceStorageMockRecorder) ReadOldSpaceHash() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadOldSpaceHash", reflect.TypeOf((*MockSpaceStorage)(nil).ReadOldSpaceHash))
}

// ReadSpaceHash mocks base method.
func (m *MockSpaceStorage) ReadSpaceHash() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadSpaceHash")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadSpaceHash indicates an expected call of ReadSpaceHash.
func (mr *MockSpaceStorageMockRecorder) ReadSpaceHash() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadSpaceHash", reflect.TypeOf((*MockSpaceStorage)(nil).ReadSpaceHash))
}

// Run mocks base method.
func (m *MockSpaceStorage) Run(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockSpaceStorageMockRecorder) Run(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockSpaceStorage)(nil).Run), arg0)
}

// SetSpaceDeleted mocks base method.
func (m *MockSpaceStorage) SetSpaceDeleted() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetSpaceDeleted")
	ret0, _ := ret[0].(error)
	return ret0
}

// SetSpaceDeleted indicates an expected call of SetSpaceDeleted.
func (mr *MockSpaceStorageMockRecorder) SetSpaceDeleted() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetSpaceDeleted", reflect.TypeOf((*MockSpaceStorage)(nil).SetSpaceDeleted))
}

// SetTreeDeletedStatus mocks base method.
func (m *MockSpaceStorage) SetTreeDeletedStatus(arg0, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetTreeDeletedStatus", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetTreeDeletedStatus indicates an expected call of SetTreeDeletedStatus.
func (mr *MockSpaceStorageMockRecorder) SetTreeDeletedStatus(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTreeDeletedStatus", reflect.TypeOf((*MockSpaceStorage)(nil).SetTreeDeletedStatus), arg0, arg1)
}

// SpaceHeader mocks base method.
func (m *MockSpaceStorage) SpaceHeader() (*spacesyncproto.RawSpaceHeaderWithId, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SpaceHeader")
	ret0, _ := ret[0].(*spacesyncproto.RawSpaceHeaderWithId)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SpaceHeader indicates an expected call of SpaceHeader.
func (mr *MockSpaceStorageMockRecorder) SpaceHeader() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SpaceHeader", reflect.TypeOf((*MockSpaceStorage)(nil).SpaceHeader))
}

// SpaceSettingsId mocks base method.
func (m *MockSpaceStorage) SpaceSettingsId() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SpaceSettingsId")
	ret0, _ := ret[0].(string)
	return ret0
}

// SpaceSettingsId indicates an expected call of SpaceSettingsId.
func (mr *MockSpaceStorageMockRecorder) SpaceSettingsId() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SpaceSettingsId", reflect.TypeOf((*MockSpaceStorage)(nil).SpaceSettingsId))
}

// StoredIds mocks base method.
func (m *MockSpaceStorage) StoredIds() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoredIds")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StoredIds indicates an expected call of StoredIds.
func (mr *MockSpaceStorageMockRecorder) StoredIds() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoredIds", reflect.TypeOf((*MockSpaceStorage)(nil).StoredIds))
}

// TreeDeletedStatus mocks base method.
func (m *MockSpaceStorage) TreeDeletedStatus(arg0 string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TreeDeletedStatus", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TreeDeletedStatus indicates an expected call of TreeDeletedStatus.
func (mr *MockSpaceStorageMockRecorder) TreeDeletedStatus(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TreeDeletedStatus", reflect.TypeOf((*MockSpaceStorage)(nil).TreeDeletedStatus), arg0)
}

// TreeRoot mocks base method.
func (m *MockSpaceStorage) TreeRoot(arg0 string) (*treechangeproto.RawTreeChangeWithId, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TreeRoot", arg0)
	ret0, _ := ret[0].(*treechangeproto.RawTreeChangeWithId)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TreeRoot indicates an expected call of TreeRoot.
func (mr *MockSpaceStorageMockRecorder) TreeRoot(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TreeRoot", reflect.TypeOf((*MockSpaceStorage)(nil).TreeRoot), arg0)
}

// TreeStorage mocks base method.
func (m *MockSpaceStorage) TreeStorage(arg0 string) (treestorage.TreeStorage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TreeStorage", arg0)
	ret0, _ := ret[0].(treestorage.TreeStorage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TreeStorage indicates an expected call of TreeStorage.
func (mr *MockSpaceStorageMockRecorder) TreeStorage(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TreeStorage", reflect.TypeOf((*MockSpaceStorage)(nil).TreeStorage), arg0)
}

// WriteOldSpaceHash mocks base method.
func (m *MockSpaceStorage) WriteOldSpaceHash(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteOldSpaceHash", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteOldSpaceHash indicates an expected call of WriteOldSpaceHash.
func (mr *MockSpaceStorageMockRecorder) WriteOldSpaceHash(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteOldSpaceHash", reflect.TypeOf((*MockSpaceStorage)(nil).WriteOldSpaceHash), arg0)
}

// WriteSpaceHash mocks base method.
func (m *MockSpaceStorage) WriteSpaceHash(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteSpaceHash", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteSpaceHash indicates an expected call of WriteSpaceHash.
func (mr *MockSpaceStorageMockRecorder) WriteSpaceHash(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteSpaceHash", reflect.TypeOf((*MockSpaceStorage)(nil).WriteSpaceHash), arg0)
}
