// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/any-sync/commonspace/object/tree/synctree (interfaces: SyncTree,HeadNotifiable,SyncClient,RequestFactory)
//
// Generated by this command:
//
//	mockgen -destination mock_synctree/mock_synctree.go github.com/anyproto/any-sync/commonspace/object/tree/synctree SyncTree,HeadNotifiable,SyncClient,RequestFactory
//

// Package mock_synctree is a generated GoMock package.
package mock_synctree

import (
	context "context"
	reflect "reflect"
	time "time"

	list "github.com/anyproto/any-sync/commonspace/object/acl/list"
	objecttree "github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	synctree "github.com/anyproto/any-sync/commonspace/object/tree/synctree"
	updatelistener "github.com/anyproto/any-sync/commonspace/object/tree/synctree/updatelistener"
	treechangeproto "github.com/anyproto/any-sync/commonspace/object/tree/treechangeproto"
	treestorage "github.com/anyproto/any-sync/commonspace/object/tree/treestorage"
	spacesyncproto "github.com/anyproto/any-sync/commonspace/spacesyncproto"
	objectmessages "github.com/anyproto/any-sync/commonspace/sync/objectsync/objectmessages"
	syncdeps "github.com/anyproto/any-sync/commonspace/sync/syncdeps"
	syncstatus "github.com/anyproto/any-sync/commonspace/syncstatus"
	peer "github.com/anyproto/any-sync/net/peer"
	proto "github.com/anyproto/protobuf/proto"
	gomock "go.uber.org/mock/gomock"
	drpc "storj.io/drpc"
)

// MockSyncTree is a mock of SyncTree interface.
type MockSyncTree struct {
	ctrl     *gomock.Controller
	recorder *MockSyncTreeMockRecorder
}

// MockSyncTreeMockRecorder is the mock recorder for MockSyncTree.
type MockSyncTreeMockRecorder struct {
	mock *MockSyncTree
}

// NewMockSyncTree creates a new mock instance.
func NewMockSyncTree(ctrl *gomock.Controller) *MockSyncTree {
	mock := &MockSyncTree{ctrl: ctrl}
	mock.recorder = &MockSyncTreeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSyncTree) EXPECT() *MockSyncTreeMockRecorder {
	return m.recorder
}

// AclList mocks base method.
func (m *MockSyncTree) AclList() list.AclList {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AclList")
	ret0, _ := ret[0].(list.AclList)
	return ret0
}

// AclList indicates an expected call of AclList.
func (mr *MockSyncTreeMockRecorder) AclList() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AclList", reflect.TypeOf((*MockSyncTree)(nil).AclList))
}

// AddContent mocks base method.
func (m *MockSyncTree) AddContent(arg0 context.Context, arg1 objecttree.SignableChangeContent) (objecttree.AddResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddContent", arg0, arg1)
	ret0, _ := ret[0].(objecttree.AddResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddContent indicates an expected call of AddContent.
func (mr *MockSyncTreeMockRecorder) AddContent(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddContent", reflect.TypeOf((*MockSyncTree)(nil).AddContent), arg0, arg1)
}

// AddRawChanges mocks base method.
func (m *MockSyncTree) AddRawChanges(arg0 context.Context, arg1 objecttree.RawChangesPayload) (objecttree.AddResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddRawChanges", arg0, arg1)
	ret0, _ := ret[0].(objecttree.AddResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddRawChanges indicates an expected call of AddRawChanges.
func (mr *MockSyncTreeMockRecorder) AddRawChanges(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRawChanges", reflect.TypeOf((*MockSyncTree)(nil).AddRawChanges), arg0, arg1)
}

// AddRawChangesFromPeer mocks base method.
func (m *MockSyncTree) AddRawChangesFromPeer(arg0 context.Context, arg1 string, arg2 objecttree.RawChangesPayload) (objecttree.AddResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddRawChangesFromPeer", arg0, arg1, arg2)
	ret0, _ := ret[0].(objecttree.AddResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddRawChangesFromPeer indicates an expected call of AddRawChangesFromPeer.
func (mr *MockSyncTreeMockRecorder) AddRawChangesFromPeer(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRawChangesFromPeer", reflect.TypeOf((*MockSyncTree)(nil).AddRawChangesFromPeer), arg0, arg1, arg2)
}

// ChangeInfo mocks base method.
func (m *MockSyncTree) ChangeInfo() *treechangeproto.TreeChangeInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChangeInfo")
	ret0, _ := ret[0].(*treechangeproto.TreeChangeInfo)
	return ret0
}

// ChangeInfo indicates an expected call of ChangeInfo.
func (mr *MockSyncTreeMockRecorder) ChangeInfo() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChangeInfo", reflect.TypeOf((*MockSyncTree)(nil).ChangeInfo))
}

// ChangesAfterCommonSnapshot mocks base method.
func (m *MockSyncTree) ChangesAfterCommonSnapshot(arg0, arg1 []string) ([]*treechangeproto.RawTreeChangeWithId, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChangesAfterCommonSnapshot", arg0, arg1)
	ret0, _ := ret[0].([]*treechangeproto.RawTreeChangeWithId)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ChangesAfterCommonSnapshot indicates an expected call of ChangesAfterCommonSnapshot.
func (mr *MockSyncTreeMockRecorder) ChangesAfterCommonSnapshot(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChangesAfterCommonSnapshot", reflect.TypeOf((*MockSyncTree)(nil).ChangesAfterCommonSnapshot), arg0, arg1)
}

// ChangesAfterCommonSnapshotLoader mocks base method.
func (m *MockSyncTree) ChangesAfterCommonSnapshotLoader(arg0, arg1 []string) (objecttree.LoadIterator, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ChangesAfterCommonSnapshotLoader", arg0, arg1)
	ret0, _ := ret[0].(objecttree.LoadIterator)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ChangesAfterCommonSnapshotLoader indicates an expected call of ChangesAfterCommonSnapshotLoader.
func (mr *MockSyncTreeMockRecorder) ChangesAfterCommonSnapshotLoader(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChangesAfterCommonSnapshotLoader", reflect.TypeOf((*MockSyncTree)(nil).ChangesAfterCommonSnapshotLoader), arg0, arg1)
}

// Close mocks base method.
func (m *MockSyncTree) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockSyncTreeMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockSyncTree)(nil).Close))
}

// Debug mocks base method.
func (m *MockSyncTree) Debug(arg0 objecttree.DescriptionParser) (objecttree.DebugInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Debug", arg0)
	ret0, _ := ret[0].(objecttree.DebugInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Debug indicates an expected call of Debug.
func (mr *MockSyncTreeMockRecorder) Debug(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Debug", reflect.TypeOf((*MockSyncTree)(nil).Debug), arg0)
}

// Delete mocks base method.
func (m *MockSyncTree) Delete() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete")
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockSyncTreeMockRecorder) Delete() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockSyncTree)(nil).Delete))
}

// GetChange mocks base method.
func (m *MockSyncTree) GetChange(arg0 string) (*objecttree.Change, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetChange", arg0)
	ret0, _ := ret[0].(*objecttree.Change)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetChange indicates an expected call of GetChange.
func (mr *MockSyncTreeMockRecorder) GetChange(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetChange", reflect.TypeOf((*MockSyncTree)(nil).GetChange), arg0)
}

// HandleDeprecatedRequest mocks base method.
func (m *MockSyncTree) HandleDeprecatedRequest(arg0 context.Context, arg1 *spacesyncproto.ObjectSyncMessage) (*spacesyncproto.ObjectSyncMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleDeprecatedRequest", arg0, arg1)
	ret0, _ := ret[0].(*spacesyncproto.ObjectSyncMessage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HandleDeprecatedRequest indicates an expected call of HandleDeprecatedRequest.
func (mr *MockSyncTreeMockRecorder) HandleDeprecatedRequest(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleDeprecatedRequest", reflect.TypeOf((*MockSyncTree)(nil).HandleDeprecatedRequest), arg0, arg1)
}

// HandleHeadUpdate mocks base method.
func (m *MockSyncTree) HandleHeadUpdate(arg0 context.Context, arg1 syncstatus.StatusUpdater, arg2 drpc.Message) (syncdeps.Request, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleHeadUpdate", arg0, arg1, arg2)
	ret0, _ := ret[0].(syncdeps.Request)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HandleHeadUpdate indicates an expected call of HandleHeadUpdate.
func (mr *MockSyncTreeMockRecorder) HandleHeadUpdate(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleHeadUpdate", reflect.TypeOf((*MockSyncTree)(nil).HandleHeadUpdate), arg0, arg1, arg2)
}

// HandleResponse mocks base method.
func (m *MockSyncTree) HandleResponse(arg0 context.Context, arg1, arg2 string, arg3 syncdeps.Response) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleResponse", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleResponse indicates an expected call of HandleResponse.
func (mr *MockSyncTreeMockRecorder) HandleResponse(arg0, arg1, arg2, arg3 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleResponse", reflect.TypeOf((*MockSyncTree)(nil).HandleResponse), arg0, arg1, arg2, arg3)
}

// HandleStreamRequest mocks base method.
func (m *MockSyncTree) HandleStreamRequest(arg0 context.Context, arg1 syncdeps.Request, arg2 syncdeps.QueueSizeUpdater, arg3 func(proto.Message) error) (syncdeps.Request, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleStreamRequest", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(syncdeps.Request)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HandleStreamRequest indicates an expected call of HandleStreamRequest.
func (mr *MockSyncTreeMockRecorder) HandleStreamRequest(arg0, arg1, arg2, arg3 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleStreamRequest", reflect.TypeOf((*MockSyncTree)(nil).HandleStreamRequest), arg0, arg1, arg2, arg3)
}

// HasChanges mocks base method.
func (m *MockSyncTree) HasChanges(arg0 ...string) bool {
	m.ctrl.T.Helper()
	varargs := []any{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "HasChanges", varargs...)
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasChanges indicates an expected call of HasChanges.
func (mr *MockSyncTreeMockRecorder) HasChanges(arg0 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasChanges", reflect.TypeOf((*MockSyncTree)(nil).HasChanges), arg0...)
}

// Header mocks base method.
func (m *MockSyncTree) Header() *treechangeproto.RawTreeChangeWithId {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Header")
	ret0, _ := ret[0].(*treechangeproto.RawTreeChangeWithId)
	return ret0
}

// Header indicates an expected call of Header.
func (mr *MockSyncTreeMockRecorder) Header() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Header", reflect.TypeOf((*MockSyncTree)(nil).Header))
}

// Heads mocks base method.
func (m *MockSyncTree) Heads() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Heads")
	ret0, _ := ret[0].([]string)
	return ret0
}

// Heads indicates an expected call of Heads.
func (mr *MockSyncTreeMockRecorder) Heads() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Heads", reflect.TypeOf((*MockSyncTree)(nil).Heads))
}

// Id mocks base method.
func (m *MockSyncTree) Id() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Id")
	ret0, _ := ret[0].(string)
	return ret0
}

// Id indicates an expected call of Id.
func (mr *MockSyncTreeMockRecorder) Id() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Id", reflect.TypeOf((*MockSyncTree)(nil).Id))
}

// IsDerived mocks base method.
func (m *MockSyncTree) IsDerived() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsDerived")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsDerived indicates an expected call of IsDerived.
func (mr *MockSyncTreeMockRecorder) IsDerived() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsDerived", reflect.TypeOf((*MockSyncTree)(nil).IsDerived))
}

// IterateFrom mocks base method.
func (m *MockSyncTree) IterateFrom(arg0 string, arg1 func(*objecttree.Change, []byte) (any, error), arg2 func(*objecttree.Change) bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IterateFrom", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// IterateFrom indicates an expected call of IterateFrom.
func (mr *MockSyncTreeMockRecorder) IterateFrom(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IterateFrom", reflect.TypeOf((*MockSyncTree)(nil).IterateFrom), arg0, arg1, arg2)
}

// IterateRoot mocks base method.
func (m *MockSyncTree) IterateRoot(arg0 func(*objecttree.Change, []byte) (any, error), arg1 func(*objecttree.Change) bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IterateRoot", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// IterateRoot indicates an expected call of IterateRoot.
func (mr *MockSyncTreeMockRecorder) IterateRoot(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IterateRoot", reflect.TypeOf((*MockSyncTree)(nil).IterateRoot), arg0, arg1)
}

// Len mocks base method.
func (m *MockSyncTree) Len() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Len")
	ret0, _ := ret[0].(int)
	return ret0
}

// Len indicates an expected call of Len.
func (mr *MockSyncTreeMockRecorder) Len() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Len", reflect.TypeOf((*MockSyncTree)(nil).Len))
}

// Lock mocks base method.
func (m *MockSyncTree) Lock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Lock")
}

// Lock indicates an expected call of Lock.
func (mr *MockSyncTreeMockRecorder) Lock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Lock", reflect.TypeOf((*MockSyncTree)(nil).Lock))
}

// PrepareChange mocks base method.
func (m *MockSyncTree) PrepareChange(arg0 objecttree.SignableChangeContent) (*treechangeproto.RawTreeChangeWithId, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PrepareChange", arg0)
	ret0, _ := ret[0].(*treechangeproto.RawTreeChangeWithId)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PrepareChange indicates an expected call of PrepareChange.
func (mr *MockSyncTreeMockRecorder) PrepareChange(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PrepareChange", reflect.TypeOf((*MockSyncTree)(nil).PrepareChange), arg0)
}

// ResponseCollector mocks base method.
func (m *MockSyncTree) ResponseCollector() syncdeps.ResponseCollector {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResponseCollector")
	ret0, _ := ret[0].(syncdeps.ResponseCollector)
	return ret0
}

// ResponseCollector indicates an expected call of ResponseCollector.
func (mr *MockSyncTreeMockRecorder) ResponseCollector() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResponseCollector", reflect.TypeOf((*MockSyncTree)(nil).ResponseCollector))
}

// Root mocks base method.
func (m *MockSyncTree) Root() *objecttree.Change {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Root")
	ret0, _ := ret[0].(*objecttree.Change)
	return ret0
}

// Root indicates an expected call of Root.
func (mr *MockSyncTreeMockRecorder) Root() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Root", reflect.TypeOf((*MockSyncTree)(nil).Root))
}

// SetListener mocks base method.
func (m *MockSyncTree) SetListener(arg0 updatelistener.UpdateListener) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetListener", arg0)
}

// SetListener indicates an expected call of SetListener.
func (mr *MockSyncTreeMockRecorder) SetListener(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetListener", reflect.TypeOf((*MockSyncTree)(nil).SetListener), arg0)
}

// SnapshotPath mocks base method.
func (m *MockSyncTree) SnapshotPath() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SnapshotPath")
	ret0, _ := ret[0].([]string)
	return ret0
}

// SnapshotPath indicates an expected call of SnapshotPath.
func (mr *MockSyncTreeMockRecorder) SnapshotPath() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SnapshotPath", reflect.TypeOf((*MockSyncTree)(nil).SnapshotPath))
}

// Storage mocks base method.
func (m *MockSyncTree) Storage() treestorage.TreeStorage {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Storage")
	ret0, _ := ret[0].(treestorage.TreeStorage)
	return ret0
}

// Storage indicates an expected call of Storage.
func (mr *MockSyncTreeMockRecorder) Storage() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Storage", reflect.TypeOf((*MockSyncTree)(nil).Storage))
}

// SyncWithPeer mocks base method.
func (m *MockSyncTree) SyncWithPeer(arg0 context.Context, arg1 peer.Peer) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SyncWithPeer", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SyncWithPeer indicates an expected call of SyncWithPeer.
func (mr *MockSyncTreeMockRecorder) SyncWithPeer(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SyncWithPeer", reflect.TypeOf((*MockSyncTree)(nil).SyncWithPeer), arg0, arg1)
}

// TryClose mocks base method.
func (m *MockSyncTree) TryClose(arg0 time.Duration) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TryClose", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TryClose indicates an expected call of TryClose.
func (mr *MockSyncTreeMockRecorder) TryClose(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TryClose", reflect.TypeOf((*MockSyncTree)(nil).TryClose), arg0)
}

// TryLock mocks base method.
func (m *MockSyncTree) TryLock() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TryLock")
	ret0, _ := ret[0].(bool)
	return ret0
}

// TryLock indicates an expected call of TryLock.
func (mr *MockSyncTreeMockRecorder) TryLock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TryLock", reflect.TypeOf((*MockSyncTree)(nil).TryLock))
}

// Unlock mocks base method.
func (m *MockSyncTree) Unlock() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Unlock")
}

// Unlock indicates an expected call of Unlock.
func (mr *MockSyncTreeMockRecorder) Unlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unlock", reflect.TypeOf((*MockSyncTree)(nil).Unlock))
}

// UnmarshalledHeader mocks base method.
func (m *MockSyncTree) UnmarshalledHeader() *objecttree.Change {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnmarshalledHeader")
	ret0, _ := ret[0].(*objecttree.Change)
	return ret0
}

// UnmarshalledHeader indicates an expected call of UnmarshalledHeader.
func (mr *MockSyncTreeMockRecorder) UnmarshalledHeader() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnmarshalledHeader", reflect.TypeOf((*MockSyncTree)(nil).UnmarshalledHeader))
}

// UnpackChange mocks base method.
func (m *MockSyncTree) UnpackChange(arg0 *treechangeproto.RawTreeChangeWithId) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnpackChange", arg0)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UnpackChange indicates an expected call of UnpackChange.
func (mr *MockSyncTreeMockRecorder) UnpackChange(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnpackChange", reflect.TypeOf((*MockSyncTree)(nil).UnpackChange), arg0)
}

// MockHeadNotifiable is a mock of HeadNotifiable interface.
type MockHeadNotifiable struct {
	ctrl     *gomock.Controller
	recorder *MockHeadNotifiableMockRecorder
}

// MockHeadNotifiableMockRecorder is the mock recorder for MockHeadNotifiable.
type MockHeadNotifiableMockRecorder struct {
	mock *MockHeadNotifiable
}

// NewMockHeadNotifiable creates a new mock instance.
func NewMockHeadNotifiable(ctrl *gomock.Controller) *MockHeadNotifiable {
	mock := &MockHeadNotifiable{ctrl: ctrl}
	mock.recorder = &MockHeadNotifiableMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHeadNotifiable) EXPECT() *MockHeadNotifiableMockRecorder {
	return m.recorder
}

// UpdateHeads mocks base method.
func (m *MockHeadNotifiable) UpdateHeads(arg0 string, arg1 []string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateHeads", arg0, arg1)
}

// UpdateHeads indicates an expected call of UpdateHeads.
func (mr *MockHeadNotifiableMockRecorder) UpdateHeads(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateHeads", reflect.TypeOf((*MockHeadNotifiable)(nil).UpdateHeads), arg0, arg1)
}

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
func (m *MockSyncClient) Broadcast(arg0 context.Context, arg1 *objectmessages.HeadUpdate) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Broadcast", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Broadcast indicates an expected call of Broadcast.
func (mr *MockSyncClientMockRecorder) Broadcast(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Broadcast", reflect.TypeOf((*MockSyncClient)(nil).Broadcast), arg0, arg1)
}

// CreateFullSyncRequest mocks base method.
func (m *MockSyncClient) CreateFullSyncRequest(arg0 string, arg1 objecttree.ObjectTree) *objectmessages.Request {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateFullSyncRequest", arg0, arg1)
	ret0, _ := ret[0].(*objectmessages.Request)
	return ret0
}

// CreateFullSyncRequest indicates an expected call of CreateFullSyncRequest.
func (mr *MockSyncClientMockRecorder) CreateFullSyncRequest(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateFullSyncRequest", reflect.TypeOf((*MockSyncClient)(nil).CreateFullSyncRequest), arg0, arg1)
}

// CreateHeadUpdate mocks base method.
func (m *MockSyncClient) CreateHeadUpdate(arg0 objecttree.ObjectTree, arg1 string, arg2 []*treechangeproto.RawTreeChangeWithId) *objectmessages.HeadUpdate {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateHeadUpdate", arg0, arg1, arg2)
	ret0, _ := ret[0].(*objectmessages.HeadUpdate)
	return ret0
}

// CreateHeadUpdate indicates an expected call of CreateHeadUpdate.
func (mr *MockSyncClientMockRecorder) CreateHeadUpdate(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateHeadUpdate", reflect.TypeOf((*MockSyncClient)(nil).CreateHeadUpdate), arg0, arg1, arg2)
}

// CreateNewTreeRequest mocks base method.
func (m *MockSyncClient) CreateNewTreeRequest(arg0, arg1 string) *objectmessages.Request {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateNewTreeRequest", arg0, arg1)
	ret0, _ := ret[0].(*objectmessages.Request)
	return ret0
}

// CreateNewTreeRequest indicates an expected call of CreateNewTreeRequest.
func (mr *MockSyncClientMockRecorder) CreateNewTreeRequest(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateNewTreeRequest", reflect.TypeOf((*MockSyncClient)(nil).CreateNewTreeRequest), arg0, arg1)
}

// CreateResponseProducer mocks base method.
func (m *MockSyncClient) CreateResponseProducer(arg0 objecttree.ObjectTree, arg1, arg2 []string) (synctree.ResponseProducer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateResponseProducer", arg0, arg1, arg2)
	ret0, _ := ret[0].(synctree.ResponseProducer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateResponseProducer indicates an expected call of CreateResponseProducer.
func (mr *MockSyncClientMockRecorder) CreateResponseProducer(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateResponseProducer", reflect.TypeOf((*MockSyncClient)(nil).CreateResponseProducer), arg0, arg1, arg2)
}

// QueueRequest mocks base method.
func (m *MockSyncClient) QueueRequest(arg0 context.Context, arg1 syncdeps.Request) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueueRequest", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// QueueRequest indicates an expected call of QueueRequest.
func (mr *MockSyncClientMockRecorder) QueueRequest(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueueRequest", reflect.TypeOf((*MockSyncClient)(nil).QueueRequest), arg0, arg1)
}

// SendTreeRequest mocks base method.
func (m *MockSyncClient) SendTreeRequest(arg0 context.Context, arg1 syncdeps.Request, arg2 syncdeps.ResponseCollector) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendTreeRequest", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendTreeRequest indicates an expected call of SendTreeRequest.
func (mr *MockSyncClientMockRecorder) SendTreeRequest(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendTreeRequest", reflect.TypeOf((*MockSyncClient)(nil).SendTreeRequest), arg0, arg1, arg2)
}

// MockRequestFactory is a mock of RequestFactory interface.
type MockRequestFactory struct {
	ctrl     *gomock.Controller
	recorder *MockRequestFactoryMockRecorder
}

// MockRequestFactoryMockRecorder is the mock recorder for MockRequestFactory.
type MockRequestFactoryMockRecorder struct {
	mock *MockRequestFactory
}

// NewMockRequestFactory creates a new mock instance.
func NewMockRequestFactory(ctrl *gomock.Controller) *MockRequestFactory {
	mock := &MockRequestFactory{ctrl: ctrl}
	mock.recorder = &MockRequestFactoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRequestFactory) EXPECT() *MockRequestFactoryMockRecorder {
	return m.recorder
}

// CreateFullSyncRequest mocks base method.
func (m *MockRequestFactory) CreateFullSyncRequest(arg0 string, arg1 objecttree.ObjectTree) *objectmessages.Request {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateFullSyncRequest", arg0, arg1)
	ret0, _ := ret[0].(*objectmessages.Request)
	return ret0
}

// CreateFullSyncRequest indicates an expected call of CreateFullSyncRequest.
func (mr *MockRequestFactoryMockRecorder) CreateFullSyncRequest(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateFullSyncRequest", reflect.TypeOf((*MockRequestFactory)(nil).CreateFullSyncRequest), arg0, arg1)
}

// CreateHeadUpdate mocks base method.
func (m *MockRequestFactory) CreateHeadUpdate(arg0 objecttree.ObjectTree, arg1 string, arg2 []*treechangeproto.RawTreeChangeWithId) *objectmessages.HeadUpdate {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateHeadUpdate", arg0, arg1, arg2)
	ret0, _ := ret[0].(*objectmessages.HeadUpdate)
	return ret0
}

// CreateHeadUpdate indicates an expected call of CreateHeadUpdate.
func (mr *MockRequestFactoryMockRecorder) CreateHeadUpdate(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateHeadUpdate", reflect.TypeOf((*MockRequestFactory)(nil).CreateHeadUpdate), arg0, arg1, arg2)
}

// CreateNewTreeRequest mocks base method.
func (m *MockRequestFactory) CreateNewTreeRequest(arg0, arg1 string) *objectmessages.Request {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateNewTreeRequest", arg0, arg1)
	ret0, _ := ret[0].(*objectmessages.Request)
	return ret0
}

// CreateNewTreeRequest indicates an expected call of CreateNewTreeRequest.
func (mr *MockRequestFactoryMockRecorder) CreateNewTreeRequest(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateNewTreeRequest", reflect.TypeOf((*MockRequestFactory)(nil).CreateNewTreeRequest), arg0, arg1)
}

// CreateResponseProducer mocks base method.
func (m *MockRequestFactory) CreateResponseProducer(arg0 objecttree.ObjectTree, arg1, arg2 []string) (synctree.ResponseProducer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateResponseProducer", arg0, arg1, arg2)
	ret0, _ := ret[0].(synctree.ResponseProducer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateResponseProducer indicates an expected call of CreateResponseProducer.
func (mr *MockRequestFactoryMockRecorder) CreateResponseProducer(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateResponseProducer", reflect.TypeOf((*MockRequestFactory)(nil).CreateResponseProducer), arg0, arg1, arg2)
}
