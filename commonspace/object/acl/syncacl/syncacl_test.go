package syncacl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/object/acl/list/mock_list"
	"github.com/anyproto/any-sync/commonspace/object/acl/syncacl/mock_syncacl"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/net/peer"
	"github.com/anyproto/any-sync/net/rpc/rpctest"
	"github.com/anyproto/any-sync/net/secureservice"
)

var ctx = context.Background()

func TestSyncAcl_SyncWithPeer(t *testing.T) {
	// this component will be rewritten, so no need for fixture now
	t.Run("sync with old peer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		acl := mock_list.NewMockAclList(ctrl)
		s := &syncAcl{AclList: acl}
		defer ctrl.Finish()
		mockClient := mock_syncacl.NewMockSyncClient(ctrl)
		s.syncClient = mockClient
		ctx := peer.CtxWithProtoVersion(ctx, secureservice.ProtoVersion)
		pr := rpctest.MockPeer{Ctx: ctx}
		retMsg := &consensusproto.LogSyncMessage{}
		mockClient.EXPECT().CreateHeadUpdate(s, nil).Return(retMsg)
		acl.EXPECT().Lock()
		acl.EXPECT().Unlock()
		mockClient.EXPECT().SendUpdate("peerId", retMsg).Return(nil)
		err := s.SyncWithPeer(ctx, &pr)
		require.NoError(t, err)
	})
	t.Run("sync with new peer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		acl := mock_list.NewMockAclList(ctrl)
		s := &syncAcl{AclList: acl}
		defer ctrl.Finish()
		mockClient := mock_syncacl.NewMockSyncClient(ctrl)
		s.syncClient = mockClient
		ctx := peer.CtxWithProtoVersion(ctx, secureservice.NewSyncProtoVersion)
		pr := rpctest.MockPeer{Ctx: ctx}
		req := &consensusproto.LogSyncMessage{}
		mockClient.EXPECT().CreateEmptyFullSyncRequest(s).Return(req)
		acl.EXPECT().Lock()
		acl.EXPECT().Unlock()
		mockClient.EXPECT().QueueRequest("peerId", req).Return(nil)
		err := s.SyncWithPeer(ctx, &pr)
		require.NoError(t, err)
	})
}
