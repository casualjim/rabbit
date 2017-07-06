// Package tasks encapsulates a dsl for defining complex workflows.
//
// You define a task by giving a name and an entry point which is a step.
// Each task has a run and a rollback method.
//
// There are 4 major types of steps:
//   * AtomicStep
//   * SequentialStep
//   * ParallelStep
//   * BranchingStep
//
// AtomicStep steps provide a single unit of work. This can do 1 or more operations as a single atomic unit.
// Sequential steps provide a framework for multiple steps to execute and are the most primitive way of composing other steps together
package tasks

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/casualjim/rabbit/eventbus"
)

// A Task encapsulates the execution of a step and provides context and configuration.
// It serves as an executor and main interface for the user of the library.
// Steps assume they are being run by a task.
type Task interface {
	Run() error
	Rollback() error
	Close() error
	Subscribe(...eventbus.EventHandler)
	Unsubscribe(...eventbus.EventHandler)
}

// A Step encapsulates a unit of work.
type Step interface {
	Run(context.Context) (context.Context, error)
	Rollback(context.Context) (context.Context, error)
}

// Branching step provides a control flow mechanism that conditionally executes the left or the right
// hand side of 2 steps
func Branching(pred func(context.Context) bool, onTrue Step, onFalse Step) Step {
	return &branchingStep{
		predicate: pred,
		left:      onTrue,
		right:     onFalse,
	}
}

type branchingStep struct {
	predicate func(context.Context) bool
	left      Step
	right     Step
	selected  Step
}

func (b *branchingStep) Run(context context.Context) (context.Context, error) {
	if b.predicate(context) {
		b.selected = b.left
	}
	b.selected = b.right
	return b.selected.Run(context)
}

func (b *branchingStep) Rollback(context context.Context) (context.Context, error) {
	if b.selected == nil {
		return context, nil
	}
	return b.selected.Rollback(context)
}

// Run handler for an atomic step
type Run func(context.Context) (context.Context, error)

// Rollback handler for an atomic step
type Rollback func(context.Context) (context.Context, error)

// Atomic step only 1 invocation of the methods happens at any given time
// the functions are guarded by a mutex
func Atomic(run Run, rollback Rollback) Step {
	return &atomicStep{run: run, rollback: rollback}
}

type atomicStep struct {
	run      Run
	rollback Rollback
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

// Pipeline executes the steps sequentially and the result of each step is passed into the next step
func Pipeline(steps ...Step) Step {
	return &pipelineStep{steps: steps, idx: -1}
}

type pipelineStep struct {
	steps []Step
	idx   int
}

func (s *pipelineStep) Run(ctx context.Context) (context.Context, error) {
	select {
	case <-ctx.Done():
		//we're done bail
		return ctx, ctx.Err()
	default:
	}

	var err error
	for i, step := range s.steps {
		ctx, err = step.Run(ctx)
		if err != nil {
			break
		}
		s.idx = i
		select {
		case <-ctx.Done():
			//we're done, bail
			return ctx, ctx.Err()
		default:
		}
	}
	return ctx, err
}

func (s *pipelineStep) Rollback(ctx context.Context) (context.Context, error) {
	if s.idx < 0 {
		return ctx, nil
	}

	var err error
	for i := s.idx; i >= 0; i-- {
		step := s.steps[i]
		ctx, err = step.Rollback(ctx)
		if err != nil {
			continue
		}
	}
	return ctx, nil
}

// Concurrent executes the steps concurrently
func Concurrent(steps ...Step) Step {
	return &concStep{
		steps: steps,
	}
}

type concStep struct {
	steps []Step
	idx   uint64
}

func (c *concStep) Run(ctx context.Context) (context.Context, error) {

	type result struct {
		ctx context.Context
		err error
		idx int
	}

	var wg sync.WaitGroup
	merge := make(chan result)
	results := make(chan []result)

	for i, step := range c.steps {
		wg.Add(1)
		go func(ctx context.Context, i int) {
			cx, err := step.Run(ctx)
			merge <- result{cx, err, i}
			atomic.SwapUint64(&c.idx, atomic.LoadUint64(&c.idx)|uint64(i))
			wg.Done()
		}(ctx, i)
	}

	go func() {
		wg.Wait()
		close(merge)
	}()

	go func(ln int) {
		collected := make([]result, ln)
		for res := range merge {
			collected[res.idx] = res
		}
		results <- collected
		close(results)
	}(len(c.steps))

	<-results
	return ctx, nil
}

func (c *concStep) Rollback(ctx context.Context) (context.Context, error) {
	idx := atomic.LoadUint64(&c.idx)

	for i := range c.steps {
		if idx&uint64(i) == 0 {
			continue
		}
		step := c.steps[i]
		_, err := step.Rollback(ctx)
		if err != nil {
			continue
		}
	}
	return ctx, nil
}
