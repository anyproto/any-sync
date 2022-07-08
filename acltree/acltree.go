package acltree

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/account"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/thread"
)

func BuildACLTree(t thread.Thread, acc *account.AccountData) (ACLTree, error) {
	// build tree from thread
	// validate snapshot
	// build aclstate -> filter tree
	// return ACLTree(aclstate+)
	return nil, nil
}

type AddResultSummary int

const (
	AddResultSummaryNothing AddResultSummary = iota
	AddResultSummaryAppend
	AddResultSummaryRebuild
)

type AddResult struct {
	AttachedChanges   []*Change
	InvalidChanges    []*Change
	UnattachedChanges []*Change

	Summary AddResultSummary
}

type ACLTree interface {
	ACLState() *ACLState
	AddChanges(changes ...*Change) (AddResult, error)
	Heads() []string
	Iterate(func(change *Change) bool)
	IterateFrom(string, func(change *Change) bool)
	HasChange(change *Change)
}
