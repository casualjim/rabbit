package steps

import (
	"context"
	"sync"

	"github.com/casualjim/rabbit/eventbus"
	"github.com/casualjim/rabbit/tasks/internal"
	"github.com/casualjim/rabbit/tasks/rollback"
)

type result struct {
	ctx context.Context
	err error
}

// PlanOption represents a configuration option for the step execution context
type PlanOption func(*Planned)

// Plan creates a new execution context for steps
func Plan(configuration ...PlanOption) *Planned {
	exec := &Planned{
		decider: rollback.Always,
		ctx:     context.Background(),
	}
	for _, conf := range configuration {
		conf(exec)
	}
	if exec.bus == nil {
		exec.bus = eventbus.NopBus
	}
	exec.ctx, exec.cancel = context.WithCancel(
		internal.SetPublisher(exec.ctx, exec.bus),
	)
	exec.step.Announce(exec.ctx)
	return exec
}

// ParentContext adds a parent context to the executor
func ParentContext(ctx context.Context) PlanOption {
	return func(e *Planned) { e.ctx = ctx }
}

// Should allows for changing the default behavior of rolling back on every error
// to a different usage pattern, like abort on every error
// or only rollback when the context was canceled or timed out
func Should(dec Decider) PlanOption {
	return func(e *Planned) { e.decider = dec }
}

// PublishTo adds an existing eventbus to the execution context
func PublishTo(bus eventbus.EventBus) PlanOption {
	return func(e *Planned) { e.bus = bus }
}

// Run the provided step as part of this executor
func Run(step Step) PlanOption {
	return func(e *Planned) { e.step = step }
}

// Planned can execute steps
type Planned struct {
	decider Decider
	bus     eventbus.EventBus
	cancel  context.CancelFunc
	ctx     context.Context
	rw      sync.Mutex
	step    Step
}

// Execute the step using the decider for rolling back or aborting
func (e *Planned) Execute() (context.Context, error) {
	e.rw.Lock()
	PublishRunEvent(e.ctx, e.step.Name(), StateProcessing)
	cx, err := e.step.Run(e.ctx)
	if err != nil {
		if IsCanceled(err) {
			PublishRunEvent(cx, e.step.Name(), StateCanceled)
		} else {
			PublishRunEvent(cx, e.step.Name(), StateFailed)
		}
		if e.decider(err) {
			PublishRollbackEvent(cx, e.step.Name(), StateProcessing)
			cx, err = e.step.Rollback(cx)
			if err != nil {
				PublishRollbackEvent(cx, e.step.Name(), StateFailed)
			} else {
				PublishRollbackEvent(cx, e.step.Name(), StateSuccess)
			}
		} else {
			PublishRollbackEvent(cx, e.step.Name(), StateSkipped)
		}
	} else {
		PublishRunEvent(cx, e.step.Name(), StateSuccess)
	}
	e.rw.Unlock()
	return cx, err
}

// Cancel the execution of the steps
func (e *Planned) Cancel() {
	if e.cancel == nil {
		return
	}
	e.cancel()
}

// Context of the executor
func (e *Planned) Context() context.Context { return e.ctx }
