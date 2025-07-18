// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/commonspace/object/tree/synctree/response (interfaces: ResponseProducer)
//
// Generated by this command:
//
//	mockgen -destination mock_response/mock_response.go github.com/anyproto/any-sync/commonspace/object/tree/synctree/response ResponseProducer
//

// Package mock_response is a generated GoMock package.
package mock_response

import (
	reflect "reflect"

	response "github.com/anyproto/any-sync/commonspace/object/tree/synctree/response"
	gomock "go.uber.org/mock/gomock"
)

// MockResponseProducer is a mock of ResponseProducer interface.
type MockResponseProducer struct {
	ctrl     *gomock.Controller
	recorder *MockResponseProducerMockRecorder
	isgomock struct{}
}

// MockResponseProducerMockRecorder is the mock recorder for MockResponseProducer.
type MockResponseProducerMockRecorder struct {
	mock *MockResponseProducer
}

// NewMockResponseProducer creates a new mock instance.
func NewMockResponseProducer(ctrl *gomock.Controller) *MockResponseProducer {
	mock := &MockResponseProducer{ctrl: ctrl}
	mock.recorder = &MockResponseProducerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResponseProducer) EXPECT() *MockResponseProducerMockRecorder {
	return m.recorder
}

// EmptyResponse mocks base method.
func (m *MockResponseProducer) EmptyResponse() (*response.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EmptyResponse")
	ret0, _ := ret[0].(*response.Response)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EmptyResponse indicates an expected call of EmptyResponse.
func (mr *MockResponseProducerMockRecorder) EmptyResponse() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EmptyResponse", reflect.TypeOf((*MockResponseProducer)(nil).EmptyResponse))
}

// NewResponse mocks base method.
func (m *MockResponseProducer) NewResponse(batchSize int) (*response.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewResponse", batchSize)
	ret0, _ := ret[0].(*response.Response)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewResponse indicates an expected call of NewResponse.
func (mr *MockResponseProducerMockRecorder) NewResponse(batchSize any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewResponse", reflect.TypeOf((*MockResponseProducer)(nil).NewResponse), batchSize)
}
