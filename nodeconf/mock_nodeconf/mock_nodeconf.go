// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/nodeconf (interfaces: Service)
//
// Generated by this command:
//
//	mockgen -destination mock_nodeconf/mock_nodeconf.go github.com/anyproto/any-sync/nodeconf Service
//

// Package mock_nodeconf is a generated GoMock package.
package mock_nodeconf

import (
	context "context"
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	nodeconf "github.com/anyproto/any-sync/nodeconf"
	chash "github.com/anyproto/go-chash"
	gomock "go.uber.org/mock/gomock"
)

// MockService is a mock of Service interface.
type MockService struct {
	ctrl     *gomock.Controller
	recorder *MockServiceMockRecorder
	isgomock struct{}
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

// CHash mocks base method.
func (m *MockService) CHash() chash.CHash {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CHash")
	ret0, _ := ret[0].(chash.CHash)
	return ret0
}

// CHash indicates an expected call of CHash.
func (mr *MockServiceMockRecorder) CHash() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CHash", reflect.TypeOf((*MockService)(nil).CHash))
}

// Close mocks base method.
func (m *MockService) Close(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockServiceMockRecorder) Close(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockService)(nil).Close), ctx)
}

// Configuration mocks base method.
func (m *MockService) Configuration() nodeconf.Configuration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Configuration")
	ret0, _ := ret[0].(nodeconf.Configuration)
	return ret0
}

// Configuration indicates an expected call of Configuration.
func (mr *MockServiceMockRecorder) Configuration() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Configuration", reflect.TypeOf((*MockService)(nil).Configuration))
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

// CoordinatorPeers mocks base method.
func (m *MockService) CoordinatorPeers() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CoordinatorPeers")
	ret0, _ := ret[0].([]string)
	return ret0
}

// CoordinatorPeers indicates an expected call of CoordinatorPeers.
func (mr *MockServiceMockRecorder) CoordinatorPeers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CoordinatorPeers", reflect.TypeOf((*MockService)(nil).CoordinatorPeers))
}

// FilePeers mocks base method.
func (m *MockService) FilePeers() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FilePeers")
	ret0, _ := ret[0].([]string)
	return ret0
}

// FilePeers indicates an expected call of FilePeers.
func (mr *MockServiceMockRecorder) FilePeers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FilePeers", reflect.TypeOf((*MockService)(nil).FilePeers))
}

// Id mocks base method.
func (m *MockService) Id() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Id")
	ret0, _ := ret[0].(string)
	return ret0
}

// Id indicates an expected call of Id.
func (mr *MockServiceMockRecorder) Id() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Id", reflect.TypeOf((*MockService)(nil).Id))
}

// Init mocks base method.
func (m *MockService) Init(a *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", a)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockServiceMockRecorder) Init(a any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockService)(nil).Init), a)
}

// IsResponsible mocks base method.
func (m *MockService) IsResponsible(spaceId string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsResponsible", spaceId)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsResponsible indicates an expected call of IsResponsible.
func (mr *MockServiceMockRecorder) IsResponsible(spaceId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsResponsible", reflect.TypeOf((*MockService)(nil).IsResponsible), spaceId)
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

// NamingNodePeers mocks base method.
func (m *MockService) NamingNodePeers() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NamingNodePeers")
	ret0, _ := ret[0].([]string)
	return ret0
}

// NamingNodePeers indicates an expected call of NamingNodePeers.
func (mr *MockServiceMockRecorder) NamingNodePeers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NamingNodePeers", reflect.TypeOf((*MockService)(nil).NamingNodePeers))
}

// NetworkCompatibilityStatus mocks base method.
func (m *MockService) NetworkCompatibilityStatus() nodeconf.NetworkCompatibilityStatus {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NetworkCompatibilityStatus")
	ret0, _ := ret[0].(nodeconf.NetworkCompatibilityStatus)
	return ret0
}

// NetworkCompatibilityStatus indicates an expected call of NetworkCompatibilityStatus.
func (mr *MockServiceMockRecorder) NetworkCompatibilityStatus() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NetworkCompatibilityStatus", reflect.TypeOf((*MockService)(nil).NetworkCompatibilityStatus))
}

// NodeIds mocks base method.
func (m *MockService) NodeIds(spaceId string) []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeIds", spaceId)
	ret0, _ := ret[0].([]string)
	return ret0
}

// NodeIds indicates an expected call of NodeIds.
func (mr *MockServiceMockRecorder) NodeIds(spaceId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeIds", reflect.TypeOf((*MockService)(nil).NodeIds), spaceId)
}

// NodeTypes mocks base method.
func (m *MockService) NodeTypes(nodeId string) []nodeconf.NodeType {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeTypes", nodeId)
	ret0, _ := ret[0].([]nodeconf.NodeType)
	return ret0
}

// NodeTypes indicates an expected call of NodeTypes.
func (mr *MockServiceMockRecorder) NodeTypes(nodeId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeTypes", reflect.TypeOf((*MockService)(nil).NodeTypes), nodeId)
}

// Partition mocks base method.
func (m *MockService) Partition(spaceId string) int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Partition", spaceId)
	ret0, _ := ret[0].(int)
	return ret0
}

// Partition indicates an expected call of Partition.
func (mr *MockServiceMockRecorder) Partition(spaceId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Partition", reflect.TypeOf((*MockService)(nil).Partition), spaceId)
}

// PaymentProcessingNodePeers mocks base method.
func (m *MockService) PaymentProcessingNodePeers() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PaymentProcessingNodePeers")
	ret0, _ := ret[0].([]string)
	return ret0
}

// PaymentProcessingNodePeers indicates an expected call of PaymentProcessingNodePeers.
func (mr *MockServiceMockRecorder) PaymentProcessingNodePeers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PaymentProcessingNodePeers", reflect.TypeOf((*MockService)(nil).PaymentProcessingNodePeers))
}

// PeerAddresses mocks base method.
func (m *MockService) PeerAddresses(peerId string) ([]string, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PeerAddresses", peerId)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// PeerAddresses indicates an expected call of PeerAddresses.
func (mr *MockServiceMockRecorder) PeerAddresses(peerId any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PeerAddresses", reflect.TypeOf((*MockService)(nil).PeerAddresses), peerId)
}

// Run mocks base method.
func (m *MockService) Run(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Run indicates an expected call of Run.
func (mr *MockServiceMockRecorder) Run(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockService)(nil).Run), ctx)
}
