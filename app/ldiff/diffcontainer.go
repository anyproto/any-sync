package ldiff

import "context"

type RemoteTypeChecker interface {
	DiffTypeCheck(ctx context.Context, diffContainer DiffContainer) (needsSync bool, diff Diff, err error)
}

type DiffContainer interface {
	DiffTypeCheck(ctx context.Context, typeChecker RemoteTypeChecker) (needsSync bool, diff Diff, err error)
	InitialDiff() Diff
	PrecalculatedDiff() Diff
	Set(elements ...Element)
	RemoveId(id string) error
}

type diffContainer struct {
	initial       *olddiff
	precalculated *diff
}

func (d *diffContainer) InitialDiff() Diff {
	return d.initial
}

func (d *diffContainer) PrecalculatedDiff() Diff {
	return d.precalculated
}

func (d *diffContainer) Set(elements ...Element) {
	d.initial.mu.Lock()
	defer d.initial.mu.Unlock()
	defer d.initial.markHashDirty()
	d.precalculated.Set(elements...)
}

func (d *diffContainer) RemoveId(id string) error {
	d.initial.mu.Lock()
	defer d.initial.mu.Unlock()
	defer d.initial.markHashDirty()
	return d.precalculated.RemoveId(id)
}

func (d *diffContainer) DiffTypeCheck(ctx context.Context, typeChecker RemoteTypeChecker) (needsSync bool, diff Diff, err error) {
	return typeChecker.DiffTypeCheck(ctx, d)
}

func NewDiffContainer(divideFactor, compareThreshold int) DiffContainer {
	newDiff := newDiff(divideFactor, compareThreshold)
	// this was for old diffs
	oldDiff := newOldDiff(16, 16, newDiff.sl)
	return &diffContainer{
		initial:       oldDiff,
		precalculated: newDiff,
	}
}
