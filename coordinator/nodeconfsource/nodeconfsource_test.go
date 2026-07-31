package nodeconfsource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/coordinator/coordinatorclient/mock_coordinatorclient"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/nodeconf"
)

func newTestSource(t *testing.T, resp *coordinatorproto.NetworkConfigurationResponse) *nodeConfSource {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	cl := mock_coordinatorclient.NewMockCoordinatorClient(ctrl)
	cl.EXPECT().NetworkConfiguration(gomock.Any(), gomock.Any()).Return(resp, nil)
	return &nodeConfSource{cl: cl}
}

func TestNodeConfSource_GetLast_FileV2(t *testing.T) {
	t.Run("maps FileV2API to NodeTypeFileV2", func(t *testing.T) {
		src := newTestSource(t, &coordinatorproto.NetworkConfigurationResponse{
			ConfigurationId: "new",
			NetworkId:       "net",
			Nodes: []*coordinatorproto.Node{
				{PeerId: "n1", Types: []coordinatorproto.NodeType{coordinatorproto.NodeType_FileV2API}},
				{PeerId: "n2", Types: []coordinatorproto.NodeType{coordinatorproto.NodeType_TreeAPI, coordinatorproto.NodeType_FileV2API}},
			},
		})
		conf, err := src.GetLast(context.Background(), "old")
		require.NoError(t, err)
		require.Len(t, conf.Nodes, 2)
		assert.Equal(t, []nodeconf.NodeType{nodeconf.NodeTypeFileV2}, conf.Nodes[0].Types)
		assert.Equal(t, []nodeconf.NodeType{nodeconf.NodeTypeTree, nodeconf.NodeTypeFileV2}, conf.Nodes[1].Types)
	})

	// Simulates an OLD client receiving a node type its proto/switch does not know:
	// the unknown enum value must be silently dropped while known types are kept,
	// so the node stays in the sync pool and routing is unchanged.
	t.Run("unknown enum type is dropped, known types preserved", func(t *testing.T) {
		const unknownType = coordinatorproto.NodeType(9999)
		src := newTestSource(t, &coordinatorproto.NetworkConfigurationResponse{
			ConfigurationId: "new",
			NetworkId:       "net",
			Nodes: []*coordinatorproto.Node{
				{PeerId: "n1", Types: []coordinatorproto.NodeType{coordinatorproto.NodeType_TreeAPI, unknownType}},
				{PeerId: "n2", Types: []coordinatorproto.NodeType{unknownType}},
			},
		})
		conf, err := src.GetLast(context.Background(), "old")
		require.NoError(t, err)
		require.Len(t, conf.Nodes, 2)
		// known type kept
		assert.Equal(t, []nodeconf.NodeType{nodeconf.NodeTypeTree}, conf.Nodes[0].Types)
		// unknown-only node ends up with no recognized role (empty), i.e. inert — no panic, no misroute
		assert.Empty(t, conf.Nodes[1].Types)
	})

	t.Run("unchanged configuration returns ErrConfigurationNotChanged", func(t *testing.T) {
		src := newTestSource(t, &coordinatorproto.NetworkConfigurationResponse{ConfigurationId: "same"})
		_, err := src.GetLast(context.Background(), "same")
		assert.ErrorIs(t, err, nodeconf.ErrConfigurationNotChanged)
	})
}
