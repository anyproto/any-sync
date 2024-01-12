package list

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/util/crypto"
)

func removeRandomElements[T any](slice []T, count int) ([]T, []T) {
	rand.Seed(time.Now().UnixNano())
	if count > len(slice) {
		count = len(slice)
	}
	removedElements := make([]T, 0, count)
	for i := 0; i < count; i++ {
		index := rand.Intn(len(slice))
		removedElements = append(removedElements, slice[index])
		slice[index] = slice[len(slice)-1]
		slice = slice[:len(slice)-1]
	}

	return removedElements, slice
}

func TestAclStatePermissionsDiff(t *testing.T) {
	st := AclState{
		statesAtRecord: map[string][]AclAccountState{},
	}
	genState := func() AclAccountState {
		_, pub, err := crypto.GenerateRandomEd25519KeyPair()
		require.NoError(t, err)
		return AclAccountState{
			PubKey:      pub,
			Permissions: AclPermissions(aclrecordproto.AclUserPermissions_Writer),
		}
	}
	var lastStates []AclAccountState
	for i := 0; i < 100; i++ {
		state := genState()
		lastStates = append(lastStates, state)
	}
	removed, lastStates := removeRandomElements(lastStates, 30)
	var firstStates []AclAccountState
	firstStates = append(firstStates, lastStates...)
	added, firstStates := removeRandomElements(firstStates, 40)
	changed, firstStates := removeRandomElements(firstStates, 20)
	for idx, _ := range changed {
		changed[idx].Permissions = AclPermissions(aclrecordproto.AclUserPermissions_Reader)
	}
	firstStates = append(firstStates, changed...)
	for idx, _ := range changed {
		changed[idx].Permissions = AclPermissions(aclrecordproto.AclUserPermissions_Writer)
	}
	firstStates = append(firstStates, removed...)
	st.statesAtRecord["first"] = firstStates
	st.statesAtRecord["last"] = lastStates
	sortFunc := func(a, b AclAccountState) int {
		if a.PubKey == b.PubKey {
			return 0
		}
		if a.PubKey.Account() < b.PubKey.Account() {
			return -1
		}
		return 1
	}
	diff, err := st.ChangedStates("first", "last")
	require.NoError(t, err)
	slices.SortFunc(diff.Removed, sortFunc)
	slices.SortFunc(diff.Added, sortFunc)
	slices.SortFunc(diff.Changed, sortFunc)
	slices.SortFunc(added, sortFunc)
	slices.SortFunc(changed, sortFunc)
	slices.SortFunc(removed, sortFunc)
	require.Equal(t, added, diff.Added)
	require.Equal(t, changed, diff.Changed)
	require.Equal(t, removed, diff.Removed)
}
