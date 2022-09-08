package storage

import "github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/aclchanges/aclpb"

type Storage interface {
	ID() (string, error)
	Header() (*aclpb.Header, error)
}
