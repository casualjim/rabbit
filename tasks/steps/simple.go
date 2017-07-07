package steps

import (
	"context"
	"sync"
)

// Zero is the zero value for a step and doesn't take any actions
var Zero Step

func noop(c context.Context) (context.Context, error) { return c, nil }

func init() {
	// eagerly create this one
	Zero = Stateless(Run(noop), Rollback(noop))
}

// StatelessAtomic step only 1 invocation of the methods happens at any given time
// the functions are guarded by a mutex
func StatelessAtomic(run Run, rollback Rollback) Step {
	return &atomicStep{simpleStep: simpleStep{run: run, rollback: rollback}}
}

// Stateless is a simple single unit of work without any state to be maintained on the step
func Stateless(run Run, rollback Rollback) Step {
	return &simpleStep{run, rollback}
}

type atomicStep struct {
	simpleStep
	sync.Mutex
}

func (a *atomicStep) Run(ctx context.Context) (context.Context, error) {
	if a.run == nil {
		return ctx, nil
	}
	a.Lock()
	nc, err := a.run(ctx)
	a.Unlock()
	return nc, err
}

func (a *atomicStep) Rollback(ctx context.Context) (context.Context, error) {
	if a.rollback == nil {
		return ctx, nil
	}
	a.Lock()
	nc, err := a.rollback(ctx)
	a.Unlock()
	return nc, err
}

type simpleStep struct {
	run      Run
	rollback Rollback
}

func (a *simpleStep) Run(ctx context.Context) (context.Context, error) {
	if a.run == nil {
		return ctx, nil
	}
	return a.run(ctx)
}

func (a *simpleStep) Rollback(ctx context.Context) (context.Context, error) {
	if a.rollback == nil {
		return ctx, nil
	}
	return a.rollback(ctx)
}
