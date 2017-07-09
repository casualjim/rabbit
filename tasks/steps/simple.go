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
	Zero = Stateless("<nop>", noop, noop)
}

// StepName represents a step name
type StepName string

// Name method to make it easier to build named steps
func (s StepName) Name() string {
	return string(s)
}

// StatelessAtomic step only 1 invocation of the methods happens at any given time
// the functions are guarded by a mutex
func StatelessAtomic(name StepName, run func(context.Context) (context.Context, error), rollback func(context.Context) (context.Context, error)) Step {
	return &atomicStep{StepName: name, simpleStep: simpleStep{run: run, rollback: rollback}}
}

// Stateless is a simple single unit of work without any state to be maintained on the step
func Stateless(name StepName, run func(context.Context) (context.Context, error), rollback func(context.Context) (context.Context, error)) Step {
	return &simpleStep{StepName: name, run: run, rollback: rollback}
}

type atomicStep struct {
	StepName
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
	StepName
	run      func(context.Context) (context.Context, error)
	rollback func(context.Context) (context.Context, error)
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
