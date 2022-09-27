package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
)

const CName = "commonspace.storage"

type Storage interface {
	storage.Provider
	StoredIds() ([]string, error)
}
