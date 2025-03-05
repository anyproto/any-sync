// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/acl (interfaces: AclService)
//
// Generated by this command:
//
//	mockgen -destination mock_acl/mock_acl.go github.com/anyproto/any-sync/acl AclService
//
// Package mock_acl is a generated GoMock package.
package mock_acl

import (
	context "context"
	reflect "reflect"

	acl "github.com/anyproto/any-sync/acl"
	app "github.com/anyproto/any-sync/app"
	list "github.com/anyproto/any-sync/commonspace/object/acl/list"
	consensusproto "github.com/anyproto/any-sync/consensus/consensusproto"
	crypto "github.com/anyproto/any-sync/util/crypto"
	gomock "go.uber.org/mock/gomock"
)

// MockAclService is a mock of AclService interface.
type MockAclService struct {
	ctrl     *gomock.Controller
	recorder *MockAclServiceMockRecorder
	isgomock struct{}
}

// MockAclServiceMockRecorder is the mock recorder for MockAclService.
type MockAclServiceMockRecorder struct {
	mock *MockAclService
}

// NewMockAclService creates a new mock instance.
func NewMockAclService(ctrl *gomock.Controller) *MockAclService {
	mock := &MockAclService{ctrl: ctrl}
	mock.recorder = &MockAclServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAclService) EXPECT() *MockAclServiceMockRecorder {
	return m.recorder
}

// AddRecord mocks base method.
func (m *MockAclService) AddRecord(ctx context.Context, spaceId string, rec *consensusproto.RawRecord, limits acl.Limits) (*consensusproto.RawRecordWithId, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddRecord", ctx, spaceId, rec, limits)
	ret0, _ := ret[0].(*consensusproto.RawRecordWithId)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddRecord indicates an expected call of AddRecord.
func (mr *MockAclServiceMockRecorder) AddRecord(ctx, spaceId, rec, limits any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRecord", reflect.TypeOf((*MockAclService)(nil).AddRecord), ctx, spaceId, rec, limits)
}

// Close mocks base method.
func (m *MockAclService) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockAclServiceMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockAclService)(nil).Close), ctx)
}

// HasRecord mocks base method.
func (m *MockAclService) HasRecord(ctx context.Context, spaceId, recordId string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasRecord", ctx, spaceId, recordId)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HasRecord indicates an expected call of HasRecord.
func (mr *MockAclServiceMockRecorder) HasRecord(ctx, spaceId, recordId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasRecord", reflect.TypeOf((*MockAclService)(nil).HasRecord), ctx, spaceId, recordId)
}

// Init mocks base method.
func (m *MockAclService) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockAclServiceMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockAclService)(nil).Init), a)
}

// Name mocks base method.
func (m *MockAclService) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockAclServiceMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockAclService)(nil).Name))
}

// OwnerPubKey mocks base method.
func (m *MockAclService) OwnerPubKey(ctx context.Context, spaceId string) (crypto.PubKey, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OwnerPubKey", ctx, spaceId)
	ret0, _ := ret[0].(crypto.PubKey)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OwnerPubKey indicates an expected call of OwnerPubKey.
func (mr *MockAclServiceMockRecorder) OwnerPubKey(ctx, spaceId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OwnerPubKey", reflect.TypeOf((*MockAclService)(nil).OwnerPubKey), ctx, spaceId)
}

// Permissions mocks base method.
func (m *MockAclService) Permissions(ctx context.Context, identity crypto.PubKey, spaceId string) (list.AclPermissions, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Permissions", ctx, identity, spaceId)
	ret0, _ := ret[0].(list.AclPermissions)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Permissions indicates an expected call of Permissions.
func (mr *MockAclServiceMockRecorder) Permissions(ctx, identity, spaceId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Permissions", reflect.TypeOf((*MockAclService)(nil).Permissions), ctx, identity, spaceId)
}

// ReadState mocks base method.
func (m *MockAclService) ReadState(ctx context.Context, spaceId string, f func(*list.AclState) error) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadState", ctx, spaceId, f)
	ret0, _ := ret[0].(error)
	return ret0
}

// ReadState indicates an expected call of ReadState.
func (mr *MockAclServiceMockRecorder) ReadState(ctx, spaceId, f any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadState", reflect.TypeOf((*MockAclService)(nil).ReadState), ctx, spaceId, f)
}

// RecordsAfter mocks base method.
func (m *MockAclService) RecordsAfter(ctx context.Context, spaceId, aclHead string) ([]*consensusproto.RawRecordWithId, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecordsAfter", ctx, spaceId, aclHead)
	ret0, _ := ret[0].([]*consensusproto.RawRecordWithId)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RecordsAfter indicates an expected call of RecordsAfter.
func (mr *MockAclServiceMockRecorder) RecordsAfter(ctx, spaceId, aclHead any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecordsAfter", reflect.TypeOf((*MockAclService)(nil).RecordsAfter), ctx, spaceId, aclHead)
}

// Run mocks base method.
func (m *MockAclService) Run(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockAclServiceMockRecorder) Run(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockAclService)(nil).Run), ctx)
}
