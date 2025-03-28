package ldiff

import (
	"context"

	"github.com/zeebo/blake3"
)

type RemoteTypeChecker interface {
	DiffTypeCheck(ctx context.Context, diffContainer DiffContainer) (needsSync bool, diff Diff, err error)
}

type DiffContainer interface {
	DiffTypeCheck(ctx context.Context, typeChecker RemoteTypeChecker) (needsSync bool, diff Diff, err error)
	OldDiff() Diff
	NewDiff() Diff
	Set(elements ...Element)
	RemoveId(id string) error
}

type diffContainer struct {
	newDiff Diff
	oldDiff Diff
}

func (d *diffContainer) NewDiff() Diff {
	return d.newDiff
}

func (d *diffContainer) OldDiff() Diff {
	return d.oldDiff
}

func (d *diffContainer) Set(elements ...Element) {
	hasher := hashersPool.Get().(*blake3.Hasher)
	defer hashersPool.Put(hasher)
	for _, el := range elements {
		hasher.Reset()
		hasher.WriteString(el.Head)
		d.newDiff.Set(Element{
			Id:   el.Id,
			Head: string(hasher.Sum(nil)),
		})
	}
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
