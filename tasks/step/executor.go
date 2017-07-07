package step

import (
	"context"
	"sync"

	"github.com/casualjim/rabbit/tasks/rollback"
)

type result struct {
	ctx context.Context
	err error
}

// WithContext creates a step executor
func WithContext(ctx context.Context) *Executor {
	return &Executor{
		ctx:     ctx,
		decider: rollback.Always,
	}
}

// Executor can execute steps
type Executor struct {
	decider Decider
	ctx     context.Context
	rw      sync.Mutex
}

// Should allows for changing the default behavior of rolling back on every error
// to a different usage pattern, like abort on every error
// or only rollback when the context was cancelled or timed out
func (e *Executor) Should(dec Decider) *Executor {
	e.decider = dec
	return e
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
