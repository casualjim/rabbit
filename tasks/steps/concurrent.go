package steps

import (
	"context"
	"sync"
	"sync/atomic"
)

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

type concres struct {
	ctx context.Context
	err error
	idx int
}

func (c *concStep) Run(ctx context.Context) (context.Context, error) {

	var wg sync.WaitGroup
	merge := make(chan concres)
	results := make(chan []concres)

	for i, step := range c.steps {
		wg.Add(1)
		go func(ctx context.Context, i int, step Step) {
			cx, err := step.Run(ctx)
			for {
				ov := atomic.LoadUint64(&c.idx)
				nv := ov | uint64(i)
				if atomic.CompareAndSwapUint64(&c.idx, ov, nv) {
					break
				}
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

	select {
	case <-ctx.Done():
		return ctx, ctx.Err()
	case <-results:
		// done
		return ctx, nil
	}
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
