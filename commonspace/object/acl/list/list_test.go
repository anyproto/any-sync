package list

import (
	"fmt"
	"github.com/anytypeio/any-sync/commonspace/object/accountdata"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAclList_BuildRoot(t *testing.T) {
	randomKeys, err := accountdata.NewRandom()
	require.NoError(t, err)
	randomAcl, err := NewTestDerivedAcl("spaceId", randomKeys)
	require.NoError(t, err)
	fmt.Println(randomAcl.Id())
}
