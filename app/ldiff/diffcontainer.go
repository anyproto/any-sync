package ldiff

import "context"

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

func NewDiffContainer(divideFactor, compareThreshold int) DiffContainer {
	newDiff := newDiff(divideFactor, compareThreshold)
	return &diffContainer{
		precalculated: newDiff,
	}
}
