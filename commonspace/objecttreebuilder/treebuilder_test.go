package objecttreebuilder

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	anystore "github.com/anyproto/any-store"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/anyproto/any-sync/commonspace/headsync/headstorage"
	"github.com/anyproto/any-sync/commonspace/headsync/headstorage/mock_headstorage"
	"github.com/anyproto/any-sync/commonspace/object/tree/objecttree"
	"github.com/anyproto/any-sync/commonspace/spacestorage/mock_spacestorage"
)

// DeriveTree refuses a child of a derived parent on the device that
// holds the parent; only "not found" lets the derive proceed.
func TestTreeBuilder_DeriveTree_ParentRule(t *testing.T) {
	ctrl := gomock.NewController(t)
	hs := mock_headstorage.NewMockHeadStorage(ctrl)
	st := mock_spacestorage.NewMockSpaceStorage(ctrl)
	st.EXPECT().HeadStorage().Return(hs).AnyTimes()
	tb := &treeBuilder{spaceStorage: st, isClosed: &atomic.Bool{}}
	payload := objecttree.ObjectTreeDerivePayload{SpaceId: "space", ChangeType: "type", ParentId: "parent"}
	ctx := context.Background()

	hs.EXPECT().GetEntry(gomock.Any(), "parent").Return(headstorage.HeadsEntry{Id: "parent", IsDerived: true}, nil)
	_, err := tb.DeriveTree(ctx, payload)
	require.ErrorIs(t, err, objecttree.ErrDerivedParent)

	boom := errors.New("boom")
	hs.EXPECT().GetEntry(gomock.Any(), "parent").Return(headstorage.HeadsEntry{}, boom)
	_, err = tb.DeriveTree(ctx, payload)
	require.ErrorIs(t, err, boom)

	hs.EXPECT().GetEntry(gomock.Any(), "parent").Return(headstorage.HeadsEntry{}, anystore.ErrDocNotFound)
	res, err := tb.DeriveTree(ctx, payload)
	require.NoError(t, err)
	require.Len(t, res.Heads, 1)

	hs.EXPECT().GetEntry(gomock.Any(), "parent").Return(headstorage.HeadsEntry{Id: "parent"}, nil)
	_, err = tb.DeriveTree(ctx, payload)
	require.NoError(t, err)
}
