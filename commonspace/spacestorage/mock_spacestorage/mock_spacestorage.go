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
	isgomock struct{}
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
func (m *MockSpaceStorage) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockSpaceStorageMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockSpaceStorage)(nil).Close), ctx)
}

// CreateTreeStorage mocks base method.
func (m *MockSpaceStorage) CreateTreeStorage(payload treestorage.TreeStorageCreatePayload) (treestorage.TreeStorage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTreeStorage", payload)
	ret0, _ := ret[0].(treestorage.TreeStorage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateTreeStorage indicates an expected call of CreateTreeStorage.
func (mr *MockSpaceStorageMockRecorder) CreateTreeStorage(payload any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTreeStorage", reflect.TypeOf((*MockSpaceStorage)(nil).CreateTreeStorage), payload)
}

// HasTree mocks base method.
func (m *MockSpaceStorage) HasTree(id string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasTree", id)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HasTree indicates an expected call of HasTree.
func (mr *MockSpaceStorageMockRecorder) HasTree(id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasTree", reflect.TypeOf((*MockSpaceStorage)(nil).HasTree), id)
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
func (m *MockSpaceStorage) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockSpaceStorageMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockSpaceStorage)(nil).Init), a)
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
func (m *MockSpaceStorage) Run(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockSpaceStorageMockRecorder) Run(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockSpaceStorage)(nil).Run), ctx)
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
func (m *MockSpaceStorage) SetTreeDeletedStatus(id, state string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetTreeDeletedStatus", id, state)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetTreeDeletedStatus indicates an expected call of SetTreeDeletedStatus.
func (mr *MockSpaceStorageMockRecorder) SetTreeDeletedStatus(id, state any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTreeDeletedStatus", reflect.TypeOf((*MockSpaceStorage)(nil).SetTreeDeletedStatus), id, state)
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
func (m *MockSpaceStorage) TreeDeletedStatus(id string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TreeDeletedStatus", id)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TreeDeletedStatus indicates an expected call of TreeDeletedStatus.
func (mr *MockSpaceStorageMockRecorder) TreeDeletedStatus(id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TreeDeletedStatus", reflect.TypeOf((*MockSpaceStorage)(nil).TreeDeletedStatus), id)
}

// TreeRoot mocks base method.
func (m *MockSpaceStorage) TreeRoot(id string) (*treechangeproto.RawTreeChangeWithId, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TreeRoot", id)
	ret0, _ := ret[0].(*treechangeproto.RawTreeChangeWithId)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TreeRoot indicates an expected call of TreeRoot.
func (mr *MockSpaceStorageMockRecorder) TreeRoot(id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TreeRoot", reflect.TypeOf((*MockSpaceStorage)(nil).TreeRoot), id)
}

// TreeStorage mocks base method.
func (m *MockSpaceStorage) TreeStorage(id string) (treestorage.TreeStorage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TreeStorage", id)
	ret0, _ := ret[0].(treestorage.TreeStorage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TreeStorage indicates an expected call of TreeStorage.
func (mr *MockSpaceStorageMockRecorder) TreeStorage(id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TreeStorage", reflect.TypeOf((*MockSpaceStorage)(nil).TreeStorage), id)
}

// WriteSpaceHash mocks base method.
func (m *MockSpaceStorage) WriteSpaceHash(hash string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteSpaceHash", hash)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteSpaceHash indicates an expected call of WriteSpaceHash.
func (mr *MockSpaceStorageMockRecorder) WriteSpaceHash(hash any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteSpaceHash", reflect.TypeOf((*MockSpaceStorage)(nil).WriteSpaceHash), hash)
}
