package data

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/core/block/editor/state"
	"time"
)

type documentStateBuilder struct {
	tree     *Tree
	aclState *ACLState // TODO: decide if this is needed or not
}

func newDocumentStateBuilder(tree *Tree, state *ACLState) *documentStateBuilder {
	return &documentStateBuilder{
		tree:     tree,
		aclState: state,
	}
}

// TODO: we should probably merge the two builders into one
func (d *documentStateBuilder) build() (s *state.State, err error) {
	var (
		startId    string
		applyRoot  bool
		st         = time.Now()
		lastChange *Change
		count      int
	)
	rootChange := d.tree.Root()
	root := state.NewDocFromSnapshot("root", rootChange.DecryptedDocumentChange.Snapshot).(*state.State)
	root.SetChangeId(rootChange.Id)

	t := d.tree
	if startId = root.ChangeId(); startId == "" {
		startId = t.RootId()
		applyRoot = true
	}

	t.Iterate(startId, func(c *Change) (isContinue bool) {
		count++
		lastChange = c
		if startId == c.Id {
			s = root.NewState()
			if applyRoot && c.DecryptedDocumentChange != nil {
				s.ApplyChangeIgnoreErr(c.DecryptedDocumentChange.Content...)
				s.SetChangeId(c.Id)
				s.AddFileKeys(c.DecryptedDocumentChange.FileKeys...)
			}
			return true
		}
		if c.DecryptedDocumentChange != nil {
			ns := s.NewState()
			ns.ApplyChangeIgnoreErr(c.DecryptedDocumentChange.Content...)
			ns.SetChangeId(c.Id)
			ns.AddFileKeys(c.DecryptedDocumentChange.FileKeys...)
			_, _, err = state.ApplyStateFastOne(ns)
			if err != nil {
				return false
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	if lastChange != nil {
		s.SetLastModified(lastChange.Content.Timestamp, lastChange.Content.Identity)
	}

	log.Infof("build state (crdt): changes: %d; dur: %v;", count, time.Since(st))
	return s, err
}
