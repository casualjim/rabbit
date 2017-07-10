package steps

import (
	"context"

	"sync"
	"sync/atomic"
	"time"

	"github.com/casualjim/rabbit/tasks/internal"
	multierror "github.com/hashicorp/go-multierror"
)

// Concurrent executes the steps concurrently
func Concurrent(name StepName, steps ...Step) Step {
	return &concStep{
		steps:    steps,
		StepName: name,
	}
}

type concStep struct {
	StepName
	steps []Step
	idx   uint64
}

type concres struct {
	ctx context.Context
	err error
	idx int
}

// Announce the step to the world
func (c *concStep) Announce(ctx context.Context) {
	c.StepName.Announce(ctx)
	pt := SetParentName(ctx, c.Name())
	for _, step := range c.steps {
		step.Announce(pt)
	}
}

// Run the concurrent step
func (c *concStep) Run(ct context.Context) (context.Context, error) {
	var wg sync.WaitGroup
	merge := make(chan concres)
	results := make(chan []concres)
	throttle := internal.GetThrottle(ct)
	ctx := SetParentName(ct, c.Name())
Outer:
	for i, step := range c.steps {
		wg.Add(1)
		if throttle > 0 {
			time.Sleep(throttle)
		}
		select {
		case <-ctx.Done():
			wg.Done()
			break Outer
		default:
		}
		go func(ctx context.Context, i int, step Step) {
			for {
				ov := atomic.LoadUint64(&c.idx)
				nv := ov | 1<<uint64(i)
				if atomic.CompareAndSwapUint64(&c.idx, ov, nv) {
					break
				}
			}
			PublishRunEvent(ctx, step.Name(), StateProcessing)
			cx, err := step.Run(ctx)
			if err != nil {
				if IsCanceled(err) {
					PublishRunEvent(ctx, step.Name(), StateCanceled)
				} else {
					PublishRunEvent(ctx, step.Name(), StateFailed)
				}
			} else {
				PublishRunEvent(ctx, step.Name(), StateSuccess)
			}
			merge <- concres{cx, err, i}
			wg.Done()
		}(ctx, i, step)
	}

	go func() {
		wg.Wait()
		close(merge)
	}()

	go func(ln int) {
		collected := make([]concres, ln)
		for res := range merge {
			collected[res.idx] = res
		}
		results <- collected
		close(results)
	}(len(c.steps))

	collected := <-results
	return ct, maybeErrors(collected)
}

func (c *concStep) Rollback(ctx context.Context) (context.Context, error) {
	set := atomic.LoadUint64(&c.idx)
	pt := SetParentName(ctx, c.Name())

	for i, step := range c.steps {
		if set&(1<<uint64(i)) == 0 {
			PublishRollbackEvent(pt, step.Name(), StateSkipped)
			continue
		}
		PublishRollbackEvent(pt, step.Name(), StateProcessing)
		_, err := step.Rollback(ctx)
		if err != nil {
			PublishRollbackEvent(pt, step.Name(), StateFailed)
			continue
		} else {
			PublishRollbackEvent(pt, step.Name(), StateSuccess)
		}
	}
	return ctx, nil
}

func maybeErrors(res []concres) error {
	var err *multierror.Error
	for _, e := range res {
		if e.err != nil {
			err = multierror.Append(err, e.err)
		}
	}
	return err.ErrorOrNil()
}
