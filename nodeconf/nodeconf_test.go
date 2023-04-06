package nodeconf

import (
	"fmt"
	"github.com/anytypeio/go-chash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
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

type testMember string

func (t testMember) Id() string {
	return string(t)
}

func (t testMember) Capacity() float64 {
	return 1
}
