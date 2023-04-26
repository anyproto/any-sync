// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anytypeio/any-sync/commonspace/objectsync (interfaces: SyncClient)

// Package mock_objectsync is a generated GoMock package.
package mock_objectsync

import (
	context "context"
	reflect "reflect"

	objecttree "github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	treechangeproto "github.com/anytypeio/any-sync/commonspace/object/tree/treechangeproto"
	objectsync "github.com/anytypeio/any-sync/commonspace/objectsync"
	spacesyncproto "github.com/anytypeio/any-sync/commonspace/spacesyncproto"
	gomock "github.com/golang/mock/gomock"
)

// MockSyncClient is a mock of SyncClient interface.
type MockSyncClient struct {
	ctrl     *gomock.Controller
	recorder *MockSyncClientMockRecorder
}

// MockSyncClientMockRecorder is the mock recorder for MockSyncClient.
type MockSyncClientMockRecorder struct {
	mock *MockSyncClient
}

// NewMockSyncClient creates a new mock instance.
func NewMockSyncClient(ctrl *gomock.Controller) *MockSyncClient {
	mock := &MockSyncClient{ctrl: ctrl}
	mock.recorder = &MockSyncClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSyncClient) EXPECT() *MockSyncClientMockRecorder {
	return m.recorder
}

// Broadcast mocks base method.
func (m *MockSyncClient) Broadcast(arg0 context.Context, arg1 *treechangeproto.TreeSyncMessage) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Broadcast", arg0, arg1)
}

// Broadcast indicates an expected call of Broadcast.
func (mr *MockSyncClientMockRecorder) Broadcast(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Broadcast", reflect.TypeOf((*MockSyncClient)(nil).Broadcast), arg0, arg1)
}

// CreateFullSyncRequest mocks base method.
func (m *MockSyncClient) CreateFullSyncRequest(arg0 objecttree.ObjectTree, arg1, arg2 []string) (*treechangeproto.TreeSyncMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateFullSyncRequest", arg0, arg1, arg2)
	ret0, _ := ret[0].(*treechangeproto.TreeSyncMessage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateFullSyncRequest indicates an expected call of CreateFullSyncRequest.
func (mr *MockSyncClientMockRecorder) CreateFullSyncRequest(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateFullSyncRequest", reflect.TypeOf((*MockSyncClient)(nil).CreateFullSyncRequest), arg0, arg1, arg2)
}

// CreateFullSyncResponse mocks base method.
func (m *MockSyncClient) CreateFullSyncResponse(arg0 objecttree.ObjectTree, arg1, arg2 []string) (*treechangeproto.TreeSyncMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateFullSyncResponse", arg0, arg1, arg2)
	ret0, _ := ret[0].(*treechangeproto.TreeSyncMessage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateFullSyncResponse indicates an expected call of CreateFullSyncResponse.
func (mr *MockSyncClientMockRecorder) CreateFullSyncResponse(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateFullSyncResponse", reflect.TypeOf((*MockSyncClient)(nil).CreateFullSyncResponse), arg0, arg1, arg2)
}

// CreateHeadUpdate mocks base method.
func (m *MockSyncClient) CreateHeadUpdate(arg0 objecttree.ObjectTree, arg1 []*treechangeproto.RawTreeChangeWithId) *treechangeproto.TreeSyncMessage {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateHeadUpdate", arg0, arg1)
	ret0, _ := ret[0].(*treechangeproto.TreeSyncMessage)
	return ret0
}

// CreateHeadUpdate indicates an expected call of CreateHeadUpdate.
func (mr *MockSyncClientMockRecorder) CreateHeadUpdate(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateHeadUpdate", reflect.TypeOf((*MockSyncClient)(nil).CreateHeadUpdate), arg0, arg1)
}

// CreateNewTreeRequest mocks base method.
func (m *MockSyncClient) CreateNewTreeRequest() *treechangeproto.TreeSyncMessage {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateNewTreeRequest")
	ret0, _ := ret[0].(*treechangeproto.TreeSyncMessage)
	return ret0
}

// CreateNewTreeRequest indicates an expected call of CreateNewTreeRequest.
func (mr *MockSyncClientMockRecorder) CreateNewTreeRequest() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateNewTreeRequest", reflect.TypeOf((*MockSyncClient)(nil).CreateNewTreeRequest))
}

// MessagePool mocks base method.
func (m *MockSyncClient) MessagePool() objectsync.MessagePool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MessagePool")
	ret0, _ := ret[0].(objectsync.MessagePool)
	return ret0
}

// MessagePool indicates an expected call of MessagePool.
func (mr *MockSyncClientMockRecorder) MessagePool() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MessagePool", reflect.TypeOf((*MockSyncClient)(nil).MessagePool))
}

// SendSync mocks base method.
func (m *MockSyncClient) SendSync(arg0 context.Context, arg1, arg2 string, arg3 *treechangeproto.TreeSyncMessage) (*spacesyncproto.ObjectSyncMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendSync", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*spacesyncproto.ObjectSyncMessage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SendSync indicates an expected call of SendSync.
func (mr *MockSyncClientMockRecorder) SendSync(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendSync", reflect.TypeOf((*MockSyncClient)(nil).SendSync), arg0, arg1, arg2, arg3)
}

// SendWithReply mocks base method.
func (m *MockSyncClient) SendWithReply(arg0 context.Context, arg1, arg2 string, arg3 *treechangeproto.TreeSyncMessage, arg4 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendWithReply", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendWithReply indicates an expected call of SendWithReply.
func (mr *MockSyncClientMockRecorder) SendWithReply(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendWithReply", reflect.TypeOf((*MockSyncClient)(nil).SendWithReply), arg0, arg1, arg2, arg3, arg4)
}
