package steps

import (
	"context"
	"sync"

	"github.com/casualjim/rabbit/eventbus"
	"github.com/casualjim/rabbit/tasks/rollback"
)

type result struct {
	ctx context.Context
	err error
}

// ExecOpt represents a configuration option for the step execution context
type ExecOpt func(*Executor)

// Execution creates a new execution context for steps
func Execution(configuration ...ExecOpt) *Executor {
	exec := &Executor{
		decider: rollback.Always,
		ctx:     context.Background(),
	}
	for _, conf := range configuration {
		conf(exec)
	}
	if exec.bus == nil {
		exec.bus = eventbus.NopBus
	}
	exec.ctx, exec.cancel = context.WithCancel(exec.ctx)
	return exec
}

// ParentContext adds a parent context to the executor
func ParentContext(ctx context.Context) ExecOpt {
	return func(e *Executor) { e.ctx = ctx }
}

// Should allows for changing the default behavior of rolling back on every error
// to a different usage pattern, like abort on every error
// or only rollback when the context was cancelled or timed out
func Should(dec Decider) ExecOpt {
	return func(e *Executor) { e.decider = dec }
}

// PublishTo adds an existing eventbus to the execution context
func PublishTo(bus eventbus.EventBus) ExecOpt {
	return func(e *Executor) { e.bus = bus }
}

// Executor can execute steps
type Executor struct {
	decider Decider
	bus     eventbus.EventBus
	cancel  context.CancelFunc
	ctx     context.Context
	rw      sync.Mutex
}

// Run the step using the decider for rolling back or aborting
func (e *Executor) Run(step Step) (context.Context, error) {
	e.rw.Lock()
	cx, err := step.Run(e.ctx)
	if err != nil {
		if e.decider(err) {
			cx, err = step.Rollback(cx)
		}
	}
	e.rw.Unlock()
	return cx, err
}

// Cancel the execution of the steps
func (e *Executor) Cancel() {
	if e.cancel == nil {
		return
	}
	e.cancel()
}

// Context of the executor
func (e *Executor) Context() context.Context { return e.ctx }
