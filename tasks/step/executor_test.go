package step_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/casualjim/rabbit/tasks/rollback"
	"github.com/casualjim/rabbit/tasks/step"
	"github.com/stretchr/testify/assert"
)

func failFn(c context.Context) (context.Context, error) { return c, assert.AnError }

func failRun() *countingStep {
	return stepRun(failFn)
}

func stepRun(fn func(context.Context) (context.Context, error)) *countingStep {
	return &countingStep{run: fn}
}

type countingStep struct {
	run           func(context.Context) (context.Context, error)
	rollback      func(context.Context) (context.Context, error)
	runCount      int64
	rollbackCount int64
}

func (c *countingStep) Run(ctx context.Context) (context.Context, error) {
	atomic.AddInt64(&c.runCount, 1)
	if c.run != nil {
		return c.run(ctx)
	}
	return ctx, nil
}

func (c *countingStep) Rollback(ctx context.Context) (context.Context, error) {
	atomic.AddInt64(&c.rollbackCount, 1)
	if c.rollback != nil {
		return c.rollback(ctx)
	}
	return ctx, nil
}

func (c *countingStep) Runs() int {
	return int(atomic.LoadInt64(&c.runCount))
}

func (c *countingStep) Rollbacks() int {
	return int(atomic.LoadInt64(&c.rollbackCount))
}

func TestExecutor_Run(t *testing.T) {
	ctx := context.Background()
	rs := &countingStep{}

	cx, err := step.WithContext(ctx).Should(rollback.Always).Run(rs)
	if assert.NoError(t, err) {
		assert.Equal(t, ctx, cx)
		assert.Equal(t, 1, rs.Runs())
		assert.Equal(t, 0, rs.Rollbacks())
	}
}

func TestExecutor_Rollback(t *testing.T) {
	ctx := context.Background()
	rs := failRun()

	cx, err := step.WithContext(ctx).Should(rollback.Always).Run(rs)
	if assert.NoError(t, err) {
		assert.Equal(t, ctx, cx)
		assert.Equal(t, 1, rs.Runs())
		assert.Equal(t, 1, rs.Rollbacks())
	}
}
