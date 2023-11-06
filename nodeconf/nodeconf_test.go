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

type testMember string

func (t testMember) Id() string {
	return string(t)
}

func (t testMember) Capacity() float64 {
	return 1
}
