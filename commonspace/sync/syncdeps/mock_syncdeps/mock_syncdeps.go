// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/commonspace/sync/syncdeps (interfaces: ObjectSyncHandler)
//
// Generated by this command:
//
//	mockgen -destination mock_syncdeps/mock_syncdeps.go github.com/anyproto/any-sync/commonspace/sync/syncdeps ObjectSyncHandler
//

// Package mock_syncdeps is a generated GoMock package.
package mock_syncdeps

import (
	context "context"
	reflect "reflect"

	spacesyncproto "github.com/anyproto/any-sync/commonspace/spacesyncproto"
	syncdeps "github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	syncstatus "github.com/anyproto/any-sync/commonspace/syncstatus"
	proto "github.com/anyproto/protobuf/proto"
	gomock "go.uber.org/mock/gomock"
	drpc "storj.io/drpc"
)

// MockObjectSyncHandler is a mock of ObjectSyncHandler interface.
type MockObjectSyncHandler struct {
	ctrl     *gomock.Controller
	recorder *MockObjectSyncHandlerMockRecorder
}

// MockObjectSyncHandlerMockRecorder is the mock recorder for MockObjectSyncHandler.
type MockObjectSyncHandlerMockRecorder struct {
	mock *MockObjectSyncHandler
}

// NewMockObjectSyncHandler creates a new mock instance.
func NewMockObjectSyncHandler(ctrl *gomock.Controller) *MockObjectSyncHandler {
	mock := &MockObjectSyncHandler{ctrl: ctrl}
	mock.recorder = &MockObjectSyncHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockObjectSyncHandler) EXPECT() *MockObjectSyncHandlerMockRecorder {
	return m.recorder
}

// HandleDeprecatedRequest mocks base method.
func (m *MockObjectSyncHandler) HandleDeprecatedRequest(arg0 context.Context, arg1 *spacesyncproto.ObjectSyncMessage) (*spacesyncproto.ObjectSyncMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleDeprecatedRequest", arg0, arg1)
	ret0, _ := ret[0].(*spacesyncproto.ObjectSyncMessage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HandleDeprecatedRequest indicates an expected call of HandleDeprecatedRequest.
func (mr *MockObjectSyncHandlerMockRecorder) HandleDeprecatedRequest(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleDeprecatedRequest", reflect.TypeOf((*MockObjectSyncHandler)(nil).HandleDeprecatedRequest), arg0, arg1)
}

// HandleHeadUpdate mocks base method.
func (m *MockObjectSyncHandler) HandleHeadUpdate(arg0 context.Context, arg1 syncstatus.StatusUpdater, arg2 drpc.Message) (syncdeps.Request, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleHeadUpdate", arg0, arg1, arg2)
	ret0, _ := ret[0].(syncdeps.Request)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HandleHeadUpdate indicates an expected call of HandleHeadUpdate.
func (mr *MockObjectSyncHandlerMockRecorder) HandleHeadUpdate(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleHeadUpdate", reflect.TypeOf((*MockObjectSyncHandler)(nil).HandleHeadUpdate), arg0, arg1, arg2)
}

// HandleResponse mocks base method.
func (m *MockObjectSyncHandler) HandleResponse(arg0 context.Context, arg1, arg2 string, arg3 syncdeps.Response) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleResponse", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleResponse indicates an expected call of HandleResponse.
func (mr *MockObjectSyncHandlerMockRecorder) HandleResponse(arg0, arg1, arg2, arg3 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleResponse", reflect.TypeOf((*MockObjectSyncHandler)(nil).HandleResponse), arg0, arg1, arg2, arg3)
}

// HandleStreamRequest mocks base method.
func (m *MockObjectSyncHandler) HandleStreamRequest(arg0 context.Context, arg1 syncdeps.Request, arg2 syncdeps.QueueSizeUpdater, arg3 func(proto.Message) error) (syncdeps.Request, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleStreamRequest", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(syncdeps.Request)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HandleStreamRequest indicates an expected call of HandleStreamRequest.
func (mr *MockObjectSyncHandlerMockRecorder) HandleStreamRequest(arg0, arg1, arg2, arg3 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleStreamRequest", reflect.TypeOf((*MockObjectSyncHandler)(nil).HandleStreamRequest), arg0, arg1, arg2, arg3)
}

// ResponseCollector mocks base method.
func (m *MockObjectSyncHandler) ResponseCollector() syncdeps.ResponseCollector {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResponseCollector")
	ret0, _ := ret[0].(syncdeps.ResponseCollector)
	return ret0
}

// ResponseCollector indicates an expected call of ResponseCollector.
func (mr *MockObjectSyncHandlerMockRecorder) ResponseCollector() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResponseCollector", reflect.TypeOf((*MockObjectSyncHandler)(nil).ResponseCollector))
}
