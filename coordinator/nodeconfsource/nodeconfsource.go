package nodeconfsource

import (
	"context"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/coordinator/coordinatorclient"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/nodeconf"
)

type NodeConfSource interface {
	app.Component
	nodeconf.Source
}

func New() NodeConfSource {
	return new(nodeConfSource)
}

type nodeConfSource struct {
	cl coordinatorclient.CoordinatorClient
}

func (n *nodeConfSource) Init(a *app.App) (err error) {
	n.cl = a.MustComponent(coordinatorclient.CName).(coordinatorclient.CoordinatorClient)
	return nil
}

func (n *nodeConfSource) Name() (name string) {
	return nodeconf.CNameSource
}

func (n *nodeConfSource) GetLast(ctx context.Context, currentId string) (c nodeconf.Configuration, err error) {
	res, err := n.cl.NetworkConfiguration(ctx, currentId)
	if err != nil {
		return
	}
	if res.ConfigurationId == currentId {
		err = nodeconf.ErrConfigurationNotChanged
		return
	}
	nodes := make([]nodeconf.Node, len(res.Nodes))
	for i, node := range res.Nodes {
		types := make([]nodeconf.NodeType, 0, len(node.Types))
		for _, nt := range node.Types {
			switch nt {
			case coordinatorproto.NodeType_FileAPI:
				types = append(types, nodeconf.NodeTypeFile)
			case coordinatorproto.NodeType_CoordinatorAPI:
				types = append(types, nodeconf.NodeTypeCoordinator)
			case coordinatorproto.NodeType_TreeAPI:
				types = append(types, nodeconf.NodeTypeTree)
			case coordinatorproto.NodeType_ConsensusAPI:
				types = append(types, nodeconf.NodeTypeConsensus)
			case coordinatorproto.NodeType_NamingNodeAPI:
				types = append(types, nodeconf.NodeTypeNamingNode)
			case coordinatorproto.NodeType_PaymentProcessingAPI:
				types = append(types, nodeconf.NodeTypePaymentProcessingNode)
			}
		}
		nodes[i] = nodeconf.Node{
			PeerId:    node.PeerId,
			Addresses: node.Addresses,
			Types:     types,
		}
	}

	return nodeconf.Configuration{
		Id:           res.ConfigurationId,
		NetworkId:    res.NetworkId,
		Nodes:        nodes,
		CreationTime: time.Unix(int64(res.CreationTimeUnix), 0),
	}, nil
}
