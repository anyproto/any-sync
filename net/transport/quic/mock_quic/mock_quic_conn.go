// Code generated by MockGen. DO NOT EDIT.
// Source: conn.go
//
// Generated by this command:
//
//	mockgen -package=mock_quic -source=conn.go -destination=mock_quic/mock_quic_conn.go connection
//
// Package mock_quic is a generated GoMock package.
package mock_quic

import (
	context "context"
	net "net"
	reflect "reflect"

	quic "github.com/quic-go/quic-go"
	gomock "go.uber.org/mock/gomock"
)

// Mockconnection is a mock of connection interface.
type Mockconnection struct {
	ctrl     *gomock.Controller
	recorder *MockconnectionMockRecorder
}

// MockconnectionMockRecorder is the mock recorder for Mockconnection.
type MockconnectionMockRecorder struct {
	mock *Mockconnection
}

// NewMockconnection creates a new mock instance.
func NewMockconnection(ctrl *gomock.Controller) *Mockconnection {
	mock := &Mockconnection{ctrl: ctrl}
	mock.recorder = &MockconnectionMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mockconnection) EXPECT() *MockconnectionMockRecorder {
	return m.recorder
}

// AcceptStream mocks base method.
func (m *Mockconnection) AcceptStream(arg0 context.Context) (*quic.Stream, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AcceptStream", arg0)
	ret0, _ := ret[0].(*quic.Stream)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AcceptStream indicates an expected call of AcceptStream.
func (mr *MockconnectionMockRecorder) AcceptStream(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AcceptStream", reflect.TypeOf((*Mockconnection)(nil).AcceptStream), arg0)
}

// CloseWithError mocks base method.
func (m *Mockconnection) CloseWithError(arg0 quic.ApplicationErrorCode, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseWithError", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseWithError indicates an expected call of CloseWithError.
func (mr *MockconnectionMockRecorder) CloseWithError(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseWithError", reflect.TypeOf((*Mockconnection)(nil).CloseWithError), arg0, arg1)
}

// Context mocks base method.
func (m *Mockconnection) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context.
func (mr *MockconnectionMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*Mockconnection)(nil).Context))
}

// LocalAddr mocks base method.
func (m *Mockconnection) LocalAddr() net.Addr {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalAddr")
	ret0, _ := ret[0].(net.Addr)
	return ret0
}

// LocalAddr indicates an expected call of LocalAddr.
func (mr *MockconnectionMockRecorder) LocalAddr() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalAddr", reflect.TypeOf((*Mockconnection)(nil).LocalAddr))
}

// OpenStreamSync mocks base method.
func (m *Mockconnection) OpenStreamSync(arg0 context.Context) (*quic.Stream, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OpenStreamSync", arg0)
	ret0, _ := ret[0].(*quic.Stream)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OpenStreamSync indicates an expected call of OpenStreamSync.
func (mr *MockconnectionMockRecorder) OpenStreamSync(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OpenStreamSync", reflect.TypeOf((*Mockconnection)(nil).OpenStreamSync), arg0)
}

// RemoteAddr mocks base method.
func (m *Mockconnection) RemoteAddr() net.Addr {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoteAddr")
	ret0, _ := ret[0].(net.Addr)
	return ret0
}

// RemoteAddr indicates an expected call of RemoteAddr.
func (mr *MockconnectionMockRecorder) RemoteAddr() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoteAddr", reflect.TypeOf((*Mockconnection)(nil).RemoteAddr))
}
