package pogrebds

import (
	"context"
	"fmt"
	"github.com/akrylysov/pogreb"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

var _ datastore.Datastore = (*Datastore)(nil)
var _ datastore.PersistentDatastore = (*Datastore)(nil)

// NewDatastore creates a new pogreb datastore.
func NewDatastore(path string, options *pogreb.Options) (d *Datastore, err error) {
	d = &Datastore{}
	if d.db, err = pogreb.Open(path, options); err != nil {
		return nil, err
	}
	return
}

type Datastore struct {
	db *pogreb.DB
}

func (d *Datastore) Get(_ context.Context, key datastore.Key) (value []byte, err error) {
	return d.db.Get(key.Bytes())
}

func (d *Datastore) Has(ctx context.Context, key datastore.Key) (exists bool, err error) {
	return d.db.Has(key.Bytes())
}

func (d *Datastore) GetSize(ctx context.Context, key datastore.Key) (size int, err error) {
	val, err := d.Get(ctx, key)
	if err != nil {
		return
	}
	return len(val), err
}

func (d *Datastore) Query(ctx context.Context, q query.Query) (query.Results, error) {
	return nil, fmt.Errorf("pogrebds datastore doesn't support queries")
}

func (d *Datastore) Put(_ context.Context, key datastore.Key, value []byte) error {
	return d.db.Put(key.Bytes(), value)
}

func (d *Datastore) Delete(_ context.Context, key datastore.Key) error {
	return d.db.Delete(key.Bytes())
}

func (d *Datastore) Sync(ctx context.Context, prefix datastore.Key) error {
	return d.db.Sync()
}

func (d *Datastore) DiskUsage(ctx context.Context) (uint64, error) {
	size, err := d.db.FileSize()
	if err != nil {
		return 0, err
	}
	return uint64(size), nil
}

func (d *Datastore) Close() error {
	return d.db.Close()
}

func (d *Datastore) Metrics() *pogreb.Metrics {
	return d.db.Metrics()
}
