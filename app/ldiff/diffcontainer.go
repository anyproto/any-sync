package ldiff

import "context"

type RemoteTypeChecker interface {
	DiffTypeCheck(ctx context.Context, diffContainer DiffContainer) (needsSync bool, diff Diff, err error)
}

type DiffContainer interface {
	DiffTypeCheck(ctx context.Context, typeChecker RemoteTypeChecker) (needsSync bool, diff Diff, err error)
	PrecalculatedDiff() Diff
	Set(elements ...Element)
	RemoveId(id string) error
}

type diffContainer struct {
	precalculated *diff
}

func (d *diffContainer) PrecalculatedDiff() Diff {
	return d.precalculated
}

func (d *diffContainer) Set(elements ...Element) {
	d.precalculated.Set(elements...)
}

func (d *diffContainer) RemoveId(id string) error {
	return d.precalculated.RemoveId(id)
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
