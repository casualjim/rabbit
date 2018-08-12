package steps

import (
	"context"
	"sync"

	"github.com/casualjim/rabbit/eventbus"
	"github.com/casualjim/rabbit/tasks/internal"
	"github.com/casualjim/rabbit/tasks/rollback"
)

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
	// we provide a callback because we can't guarantee the events will arrive
	// in order in the handlers because they are executed in their own go routine
	// so we collect the step names through a callback which will guarantee
	// breadth first traversal of the tree.
	exec.step.Announce(exec.ctx, func(stepName string) {
		exec.stepNames = append(exec.stepNames, stepName)
	})
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
	decider   Decider
	bus       eventbus.EventBus
	cancel    context.CancelFunc
	ctx       context.Context
	rw        sync.Mutex
	decl      sync.Mutex
	step      Step
	stepNames []string
}

// Execute the step using the decider for rolling back or aborting
func (e *Planned) Execute() (context.Context, error) {
	e.rw.Lock()
	PublishRunEvent(e.ctx, e.step.Name(), StateProcessing, nil)
	cx, err := e.step.Run(e.ctx)
	if err != nil {
		if IsCanceled(err) {
			PublishRunEvent(cx, e.step.Name(), StateCanceled, nil)
		} else {
			PublishRunEvent(cx, e.step.Name(), StateFailed, err)
		}
		e.decl.Lock()
		decider := e.decider
		e.decl.Unlock()
		if decider(err) {
			PublishRollbackEvent(cx, e.step.Name(), StateProcessing, nil)
			cx, err = e.step.Rollback(cx)
			if err != nil {
				PublishRollbackEvent(cx, e.step.Name(), StateFailed, err)
			} else {
				PublishRollbackEvent(cx, e.step.Name(), StateSuccess, nil)
			}
		} else {
			PublishRollbackEvent(cx, e.step.Name(), StateSkipped, nil)
		}
	} else {
		PublishRunEvent(cx, e.step.Name(), StateSuccess, nil)
	}
	e.rw.Unlock()
	return cx, err
}

// StepNames is an ordered list with all the steps known to this plan
func (e *Planned) StepNames() []string {
	return e.stepNames
}

// Cancel the execution of the steps
func (e *Planned) Cancel(decider Decider) {
	if e.cancel == nil {
		return
	}
	if decider != nil {
		e.decl.Lock()
		e.decider = decider
		e.decl.Unlock()
	}
	e.cancel()
}

// Context of the executor
func (e *Planned) Context() context.Context { return e.ctx }
