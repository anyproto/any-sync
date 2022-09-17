package storage

import (
	"github.com/anytypeio/go-anytype-infrastructure-experiments/pkg/acl/storage"
)

type Storage interface {
	storage.Provider
}

const CName = "commonspace.storage"
