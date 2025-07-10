package ldiff

import (
	"context"
	"encoding/hex"

	"github.com/zeebo/blake3"
)

type RemoteTypeChecker interface {
	DiffTypeCheck(ctx context.Context, diffContainer DiffContainer) (needsSync bool, diff Diff, err error)
}

type DiffContainer interface {
	DiffTypeCheck(ctx context.Context, typeChecker RemoteTypeChecker) (needsSync bool, diff Diff, err error)
	OldDiff() Diff
	NewDiff() Diff
	RemoveId(id string) error
}

type diffContainer struct {
	newDiff Diff
	oldDiff Diff
}

type Hasher struct {
	hasher *blake3.Hasher
}

func (h *Hasher) HashId(id string) string {
	h.hasher.Reset()
	h.hasher.WriteString(id)
	return hex.EncodeToString(h.hasher.Sum(nil))
}

func NewHasher() *Hasher {
	return &Hasher{hashersPool.Get().(*blake3.Hasher)}
}

func ReleaseHasher(hasher *Hasher) {
	hashersPool.Put(hasher.hasher)
}

func (d *diffContainer) NewDiff() Diff {
	return d.newDiff
}

func (d *diffContainer) OldDiff() Diff {
	return d.oldDiff
}

func (d *diffContainer) Set(elements ...Element) {
	d.newDiff.Set(elements...)
	d.oldDiff.Set(elements...)
}

func (d *diffContainer) RemoveId(id string) error {
	_ = d.newDiff.RemoveId(id)
	_ = d.oldDiff.RemoveId(id)
	return nil
}

func (d *diffContainer) DiffTypeCheck(ctx context.Context, typeChecker RemoteTypeChecker) (needsSync bool, diff Diff, err error) {
	return typeChecker.DiffTypeCheck(ctx, d)
}

func NewDiffContainer(new, old Diff) DiffContainer {
	return &diffContainer{
		newDiff: new,
		oldDiff: old,
	}
}
