package list

import "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/tree"

type IterFunc = func(record *Record) (IsContinue bool)

type ACLList interface {
	tree.RWLocker
	ID() string
	ACLState() ACLState
	IsAfter(first string, second string) (bool, error)
	Last() *Record
	Get(id string) (*Record, error)
	Iterate(iterFunc IterFunc)
	IterateFrom(startId string, iterFunc IterFunc)
}
