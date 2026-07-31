package nodeconf

import "context"

const CNameStore = "common.nodeconf.store"

type Store interface {
	GetLast(ctx context.Context, netId string) (c Configuration, err error)
	SaveLast(ctx context.Context, c Configuration) (err error)
}

// HistoryStore is an optional extension of Store that additionally retains
// previously applied configurations keyed by epoch. It allows a node restarted
// mid-migration to recompute the ring of an earlier epoch.
type HistoryStore interface {
	Store
	// GetByEpoch returns a previously saved configuration of the given network.
	// Returns ErrConfigurationNotFound if the epoch is not retained.
	GetByEpoch(ctx context.Context, netId string, epoch uint64) (c Configuration, err error)
	// Epochs returns the retained epochs for the given network in ascending order.
	Epochs(ctx context.Context, netId string) (epochs []uint64, err error)
}
