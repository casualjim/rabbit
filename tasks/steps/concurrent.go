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

// Run the concurrent step
func (c *concStep) Run(ctx context.Context) (context.Context, error) {
	var wg sync.WaitGroup
	merge := make(chan concres)
	results := make(chan []concres)
	throttle := internal.GetThrottle(ctx)
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
			cx, err := step.Run(ctx)
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
	return ctx, maybeErrors(collected)
}

func (c *concStep) Rollback(ctx context.Context) (context.Context, error) {
	set := atomic.LoadUint64(&c.idx)

	for i := range c.steps {
		if set&(1<<uint64(i)) == 0 {
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

func maybeErrors(res []concres) error {
	var err error
	for _, e := range res {
		if e.err != nil {
			err = multierror.Append(err, e.err)
		}
	}
	return err
}
