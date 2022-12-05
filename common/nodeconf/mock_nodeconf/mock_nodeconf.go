// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf (interfaces: Service,Configuration,ConfConnector)

// Package mock_nodeconf is a generated GoMock package.
package mock_nodeconf

import (
	context "context"
	reflect "reflect"

	app "github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	peer "github.com/anytypeio/go-anytype-infrastructure-experiments/common/net/peer"
	nodeconf "github.com/anytypeio/go-anytype-infrastructure-experiments/common/nodeconf"
	gomock "github.com/golang/mock/gomock"
)

// MockService is a mock of Service interface.
type MockService struct {
	ctrl     *gomock.Controller
	recorder *MockServiceMockRecorder
}

// MockServiceMockRecorder is the mock recorder for MockService.
type MockServiceMockRecorder struct {
	mock *MockService
}

// NewMockService creates a new mock instance.
func NewMockService(ctrl *gomock.Controller) *MockService {
	mock := &MockService{ctrl: ctrl}
	mock.recorder = &MockServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockService) EXPECT() *MockServiceMockRecorder {
	return m.recorder
}

// ConsensusPeers mocks base method.
func (m *MockService) ConsensusPeers() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ConsensusPeers")
	ret0, _ := ret[0].([]string)
	return ret0
}

// ConsensusPeers indicates an expected call of ConsensusPeers.
func (mr *MockServiceMockRecorder) ConsensusPeers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ConsensusPeers", reflect.TypeOf((*MockService)(nil).ConsensusPeers))
}

// GetById mocks base method.
func (m *MockService) GetById(arg0 string) nodeconf.Configuration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetById", arg0)
	ret0, _ := ret[0].(nodeconf.Configuration)
	return ret0
}

// GetById indicates an expected call of GetById.
func (mr *MockServiceMockRecorder) GetById(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetById", reflect.TypeOf((*MockService)(nil).GetById), arg0)
}

// GetLast mocks base method.
func (m *MockService) GetLast() nodeconf.Configuration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLast")
	ret0, _ := ret[0].(nodeconf.Configuration)
	return ret0
}

// GetLast indicates an expected call of GetLast.
func (mr *MockServiceMockRecorder) GetLast() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLast", reflect.TypeOf((*MockService)(nil).GetLast))
}

// Init mocks base method.
func (m *MockService) Init(arg0 *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockServiceMockRecorder) Init(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockService)(nil).Init), arg0)
}

// Name mocks base method.
func (m *MockService) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockServiceMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockService)(nil).Name))
}

// MockConfiguration is a mock of Configuration interface.
type MockConfiguration struct {
	ctrl     *gomock.Controller
	recorder *MockConfigurationMockRecorder
}

// MockConfigurationMockRecorder is the mock recorder for MockConfiguration.
type MockConfigurationMockRecorder struct {
	mock *MockConfiguration
}

// NewMockConfiguration creates a new mock instance.
func NewMockConfiguration(ctrl *gomock.Controller) *MockConfiguration {
	mock := &MockConfiguration{ctrl: ctrl}
	mock.recorder = &MockConfigurationMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConfiguration) EXPECT() *MockConfigurationMockRecorder {
	return m.recorder
}

// Id mocks base method.
func (m *MockConfiguration) Id() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Id")
	ret0, _ := ret[0].(string)
	return ret0
}

// Id indicates an expected call of Id.
func (mr *MockConfigurationMockRecorder) Id() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Id", reflect.TypeOf((*MockConfiguration)(nil).Id))
}

// IsResponsible mocks base method.
func (m *MockConfiguration) IsResponsible(arg0 string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsResponsible", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsResponsible indicates an expected call of IsResponsible.
func (mr *MockConfigurationMockRecorder) IsResponsible(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsResponsible", reflect.TypeOf((*MockConfiguration)(nil).IsResponsible), arg0)
}

// NodeIds mocks base method.
func (m *MockConfiguration) NodeIds(arg0 string) []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeIds", arg0)
	ret0, _ := ret[0].([]string)
	return ret0
}

// NodeIds indicates an expected call of NodeIds.
func (mr *MockConfigurationMockRecorder) NodeIds(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeIds", reflect.TypeOf((*MockConfiguration)(nil).NodeIds), arg0)
}

// MockConfConnector is a mock of ConfConnector interface.
type MockConfConnector struct {
	ctrl     *gomock.Controller
	recorder *MockConfConnectorMockRecorder
}

// MockConfConnectorMockRecorder is the mock recorder for MockConfConnector.
type MockConfConnectorMockRecorder struct {
	mock *MockConfConnector
}

// NewMockConfConnector creates a new mock instance.
func NewMockConfConnector(ctrl *gomock.Controller) *MockConfConnector {
	mock := &MockConfConnector{ctrl: ctrl}
	mock.recorder = &MockConfConnectorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConfConnector) EXPECT() *MockConfConnectorMockRecorder {
	return m.recorder
}

// Configuration mocks base method.
func (m *MockConfConnector) Configuration() nodeconf.Configuration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Configuration")
	ret0, _ := ret[0].(nodeconf.Configuration)
	return ret0
}

// Configuration indicates an expected call of Configuration.
func (mr *MockConfConnectorMockRecorder) Configuration() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Configuration", reflect.TypeOf((*MockConfConnector)(nil).Configuration))
}

// DialInactiveResponsiblePeers mocks base method.
func (m *MockConfConnector) DialInactiveResponsiblePeers(arg0 context.Context, arg1 string, arg2 []string) ([]peer.Peer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DialInactiveResponsiblePeers", arg0, arg1, arg2)
	ret0, _ := ret[0].([]peer.Peer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DialInactiveResponsiblePeers indicates an expected call of DialInactiveResponsiblePeers.
func (mr *MockConfConnectorMockRecorder) DialInactiveResponsiblePeers(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DialInactiveResponsiblePeers", reflect.TypeOf((*MockConfConnector)(nil).DialInactiveResponsiblePeers), arg0, arg1, arg2)
}

// GetResponsiblePeers mocks base method.
func (m *MockConfConnector) GetResponsiblePeers(arg0 context.Context, arg1 string) ([]peer.Peer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetResponsiblePeers", arg0, arg1)
	ret0, _ := ret[0].([]peer.Peer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetResponsiblePeers indicates an expected call of GetResponsiblePeers.
func (mr *MockConfConnectorMockRecorder) GetResponsiblePeers(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetResponsiblePeers", reflect.TypeOf((*MockConfConnector)(nil).GetResponsiblePeers), arg0, arg1)
}
