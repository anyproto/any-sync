// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anytypeio/any-sync/commonspace/object/treegetter (interfaces: TreeGetter)

// Package mock_treegetter is a generated GoMock package.
package mock_treegetter

import (
	context "context"
	reflect "reflect"

	app "github.com/anytypeio/any-sync/app"
	objecttree "github.com/anytypeio/any-sync/commonspace/object/tree/objecttree"
	gomock "github.com/golang/mock/gomock"
)

// MockTreeGetter is a mock of TreeGetter interface.
type MockTreeGetter struct {
	ctrl     *gomock.Controller
	recorder *MockTreeGetterMockRecorder
}

// MockTreeGetterMockRecorder is the mock recorder for MockTreeGetter.
type MockTreeGetterMockRecorder struct {
	mock *MockTreeGetter
}

// NewMockTreeGetter creates a new mock instance.
func NewMockTreeGetter(ctrl *gomock.Controller) *MockTreeGetter {
	mock := &MockTreeGetter{ctrl: ctrl}
	mock.recorder = &MockTreeGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTreeGetter) EXPECT() *MockTreeGetterMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockTreeGetter) Close(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockTreeGetterMockRecorder) Close(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockTreeGetter)(nil).Close), arg0)
}

// DeleteTree mocks base method.
func (m *MockTreeGetter) DeleteTree(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteTree", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteTree indicates an expected call of DeleteTree.
func (mr *MockTreeGetterMockRecorder) DeleteTree(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteTree", reflect.TypeOf((*MockTreeGetter)(nil).DeleteTree), arg0, arg1, arg2)
}

// GetTree mocks base method.
func (m *MockTreeGetter) GetTree(arg0 context.Context, arg1, arg2 string) (objecttree.ObjectTree, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTree", arg0, arg1, arg2)
	ret0, _ := ret[0].(objecttree.ObjectTree)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTree indicates an expected call of GetTree.
func (mr *MockTreeGetterMockRecorder) GetTree(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTree", reflect.TypeOf((*MockTreeGetter)(nil).GetTree), arg0, arg1, arg2)
}

// Init mocks base method.
func (m *MockTreeGetter) Init(arg0 *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockTreeGetterMockRecorder) Init(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockTreeGetter)(nil).Init), arg0)
}

// Name mocks base method.
func (m *MockTreeGetter) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockTreeGetterMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockTreeGetter)(nil).Name))
}

// Run mocks base method.
func (m *MockTreeGetter) Run(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockTreeGetterMockRecorder) Run(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockTreeGetter)(nil).Run), arg0)
}