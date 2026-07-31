package nodeconf

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/anyproto/go-chash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"gopkg.in/yaml.v3"
)

func TestReplKey(t *testing.T) {
	assert.Equal(t, "repl", ReplKey("id.repl"))
	assert.Equal(t, "repl", ReplKey("repl"))
	assert.Equal(t, "", ReplKey("."))
}

func TestConfiguration_NodeIds(t *testing.T) {
	ch, err := chash.New(chash.Config{
		PartitionCount:    PartitionCount,
		ReplicationFactor: ReplicationFactor,
	})
	require.NoError(t, err)
	conf := &nodeConf{
		id:        "last",
		accountId: "1",
		chash:     ch,
	}
	for i := 0; i < 10; i++ {
		require.NoError(t, conf.chash.AddMembers(testMember(fmt.Sprint(i+1))))
	}

	t.Run("random keys", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			spaceId := fmt.Sprintf("%d.%d", rand.Int(), rand.Int())
			members := conf.NodeIds(spaceId)
			if conf.IsResponsible(spaceId) {
				assert.Len(t, members, 2)
			} else {
				assert.Len(t, members, 3)
			}
		}
	})
	t.Run("same repl key", func(t *testing.T) {
		var prevMemb []string
		for i := 0; i < 10; i++ {
			spaceId := fmt.Sprintf("%d.%d", rand.Int(), 42)
			members := conf.NodeIds(spaceId)
			if conf.IsResponsible(spaceId) {
				assert.Len(t, members, 2)
			} else {
				assert.Len(t, members, 3)
			}
			if i != 0 {
				assert.Equal(t, prevMemb, members)
			}
			prevMemb = members
		}
	})

}

func TestNodeConf_NamingNodePeers(t *testing.T) {
	ch, err := chash.New(chash.Config{
		PartitionCount:    PartitionCount,
		ReplicationFactor: ReplicationFactor,
	})
	require.NoError(t, err)

	conf := &nodeConf{
		id:        "last",
		accountId: "1",
		chash:     ch,
		namingNodePeers: []string{
			"naming-node-1",
			"naming-node-2",
			"naming-node-3",
		},
	}

	assert.Equal(t, []string{"naming-node-1", "naming-node-2", "naming-node-3"}, conf.NamingNodePeers())
}

func TestNodeConf_PaymentProcessingNodePeers(t *testing.T) {
	ch, err := chash.New(chash.Config{
		PartitionCount:    PartitionCount,
		ReplicationFactor: ReplicationFactor,
	})
	require.NoError(t, err)

	conf := &nodeConf{
		id:        "last",
		accountId: "1",
		chash:     ch,
		paymentProcessingNodePeers: []string{
			"payment-processing-node-1",
			"payment-processing-node-2",
			"payment-processing-node-3",
		},
	}

	assert.Equal(t, []string{"payment-processing-node-1", "payment-processing-node-2", "payment-processing-node-3"}, conf.PaymentProcessingNodePeers())
}

func TestNewNodeConfFromYaml(t *testing.T) {
	yamlData := `
id: 64ba63209976be4a733bbb91
networkId: N4Gvo3v5wL31RrYgX3PrhAGMYvdWe5rAgtVB8cZySYWrkhb6
nodes:
  - peerId: 12D3KooWA8EXV3KjBxEU5EnsPfneLx84vMWAtTBQBeyooN82KSuS
    addresses:
      - 127.0.0.1:4830
    types:
      - coordinator
  - peerId: 12D3KooWA8EXV3KjBxEU5EnsPfneLx84vMWAtTBQBeyooN82KSuS
    addresses:
      - 127.0.0.1:4830
    types:
      - namingNode
  - peerId: 12D3KooXXXEXV3KjBxEU5EnsPfneLx84vMWAtTBQBeyooN82KSuS
    addresses:
      - 127.0.0.1:4830
    types:
      - paymentProcessingNode
  - peerId: 12D3KooYYYEXV3KjBxEU5EnsPfneLx84vMWAtTBQBeyooN82KSuS
    addresses:
      - 127.0.0.1:4830
    types:
      - file
  - peerId: consensus1
    addresses:
      - 127.0.0.1:4830
    types:
      - consensus
  - peerId: consensus2
    addresses:
      - 127.0.0.1:4830
    types:
      - consensus
creationTime: 2023-07-21T11:51:12.970882+01:00
`

	var conf Configuration
	err := yaml.Unmarshal([]byte(yamlData), &conf)
	require.NoError(t, err)

	log.Info("conf", zap.Any("conf", conf))

	nodeConf, err := сonfigurationToNodeConf(conf)
	require.NoError(t, err)

	assert.Equal(t, "64ba63209976be4a733bbb91", nodeConf.Id())
	assert.Equal(t, "N4Gvo3v5wL31RrYgX3PrhAGMYvdWe5rAgtVB8cZySYWrkhb6", nodeConf.c.NetworkId)

	// should not be set by сonfigurationToNodeConf
	assert.Equal(t, "", nodeConf.accountId)
	assert.Equal(t, []string{"12D3KooYYYEXV3KjBxEU5EnsPfneLx84vMWAtTBQBeyooN82KSuS"}, nodeConf.FilePeers())
	assert.Equal(t, []string{"consensus1", "consensus2"}, nodeConf.ConsensusPeers())
	assert.Equal(t, []string{"12D3KooWA8EXV3KjBxEU5EnsPfneLx84vMWAtTBQBeyooN82KSuS"}, nodeConf.CoordinatorPeers())
	assert.Equal(t, []string{"12D3KooWA8EXV3KjBxEU5EnsPfneLx84vMWAtTBQBeyooN82KSuS"}, nodeConf.NamingNodePeers())
	assert.Equal(t, []string{"12D3KooXXXEXV3KjBxEU5EnsPfneLx84vMWAtTBQBeyooN82KSuS"}, nodeConf.PaymentProcessingNodePeers())
}

func TestConfiguration_FileV2NodeIds(t *testing.T) {
	chFileV2, err := chash.New(chash.Config{
		PartitionCount:    PartitionCount,
		ReplicationFactor: FileV2ReplicationFactor,
	})
	require.NoError(t, err)
	conf := &nodeConf{
		id:          "last",
		accountId:   "1",
		chashFileV2: chFileV2,
	}
	for i := 0; i < 5; i++ {
		require.NoError(t, conf.chashFileV2.AddMembers(testMember(fmt.Sprint(i+1))))
	}

	t.Run("returns exactly RF=2 responsible nodes", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			spaceId := fmt.Sprintf("%d.%d", rand.Int(), rand.Int())
			assert.Len(t, conf.FileV2NodeIds(spaceId), FileV2ReplicationFactor)
		}
	})

	t.Run("stable for same repl key", func(t *testing.T) {
		var prev []string
		for i := 0; i < 10; i++ {
			spaceId := fmt.Sprintf("%d.%d", rand.Int(), 42)
			members := conf.FileV2NodeIds(spaceId)
			assert.Len(t, members, FileV2ReplicationFactor)
			if i != 0 {
				assert.Equal(t, prev, members)
			}
			prev = members
		}
	})

	t.Run("does not exclude self (full pair returned)", func(t *testing.T) {
		// unlike NodeIds, the current account stays in the result: find a space for
		// which account "1" is responsible and assert it is still present.
		selfSeen := false
		for i := 0; i < 2000 && !selfSeen; i++ {
			members := conf.FileV2NodeIds(fmt.Sprintf("%d.%d", i, i))
			require.Len(t, members, FileV2ReplicationFactor)
			for _, m := range members {
				if m == conf.accountId {
					selfSeen = true
				}
			}
		}
		assert.True(t, selfSeen, "self must be included when responsible")
	})
}

func TestNewNodeConfFromYaml_FileV2(t *testing.T) {
	yamlData := `
id: net1
networkId: net1
nodes:
  - peerId: file-v1
    addresses: [127.0.0.1:1]
    types: [file]
  - peerId: filev2-1
    addresses: [127.0.0.1:2]
    types: [fileV2]
  - peerId: filev2-2
    addresses: [127.0.0.1:3]
    types: [fileV2]
  - peerId: tree-and-filev2
    addresses: [127.0.0.1:4]
    types: [tree, fileV2]
  - peerId: tree-1
    addresses: [127.0.0.1:5]
    types: [tree]
creationTime: 2023-07-21T11:51:12.970882+01:00
`
	var conf Configuration
	require.NoError(t, yaml.Unmarshal([]byte(yamlData), &conf))
	nc, err := сonfigurationToNodeConf(conf)
	require.NoError(t, err)

	// v2 pool = exactly the fileV2-typed nodes; the v1 file node is excluded
	assert.Equal(t, []string{"filev2-1", "filev2-2", "tree-and-filev2"}, nc.FileV2Peers())
	// v1 file pool unaffected by the new type
	assert.Equal(t, []string{"file-v1"}, nc.FilePeers())

	// resolution returns RF=2 responsible nodes, all from the v2 pool
	v2set := map[string]bool{"filev2-1": true, "filev2-2": true, "tree-and-filev2": true}
	ids := nc.FileV2NodeIds("space.repl")
	assert.Len(t, ids, FileV2ReplicationFactor)
	for _, id := range ids {
		assert.Truef(t, v2set[id], "resolved node %s must be a v2 file node", id)
	}

	// the tree/sync container is unaffected: it resolves only from tree-typed nodes
	treeSet := map[string]bool{"tree-and-filev2": true, "tree-1": true}
	for _, id := range nc.NodeIds("space.repl") {
		assert.Truef(t, treeSet[id], "sync node %s must be a tree node", id)
	}
}

func TestNewNodeConfFromYaml_NoFileV2(t *testing.T) {
	// a config without any fileV2 nodes must still build and resolve to an empty pair
	yamlData := `
id: net1
networkId: net1
nodes:
  - peerId: tree-1
    addresses: [127.0.0.1:1]
    types: [tree]
  - peerId: file-v1
    addresses: [127.0.0.1:2]
    types: [file]
creationTime: 2023-07-21T11:51:12.970882+01:00
`
	var conf Configuration
	require.NoError(t, yaml.Unmarshal([]byte(yamlData), &conf))
	nc, err := сonfigurationToNodeConf(conf)
	require.NoError(t, err)
	assert.Empty(t, nc.FileV2Peers())
	assert.Empty(t, nc.FileV2NodeIds("space.repl"))
}

type testMember string

func (t testMember) Id() string {
	return string(t)
}

func (t testMember) Capacity() float64 {
	return 1
}
