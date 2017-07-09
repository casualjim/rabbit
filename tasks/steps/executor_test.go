package steps_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/casualjim/rabbit/eventbus"
	"github.com/casualjim/rabbit/tasks/rollback"
	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/stretchr/testify/assert"
)

func failFn(c context.Context) (context.Context, error) { return c, assert.AnError }
func noopFn(c context.Context) (context.Context, error) { return c, nil }

func failRun(name steps.StepName) *countingStep {
	return stepRun(name, failFn)
}

func stepRun(name steps.StepName, fn func(context.Context) (context.Context, error)) *countingStep {
	return &countingStep{StepName: name, run: fn}
}

type countingStep struct {
	steps.StepName
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
	rs := &countingStep{StepName: steps.StepName("run")}

	assert.NotPanics(t, (&(steps.Executor{})).Cancel)

	exec := steps.Execution(
		steps.Should(rollback.Always),
		steps.PublishTo(eventbus.NopBus),
	)
	ctx := exec.Context()
	cx, err := exec.Run(rs)

	if assert.NoError(t, err) {
		assert.Equal(t, ctx, cx)
		assert.Equal(t, 1, rs.Runs())
		assert.Equal(t, 0, rs.Rollbacks())
	}
}

func TestExecutor_Rollback(t *testing.T) {
	rs := failRun("rollback")

	executor := steps.Execution(
		steps.Should(rollback.Always),
		steps.PublishTo(eventbus.NopBus),
	)
	ctx := executor.Context()
	cx, err := executor.Run(rs)
	if assert.NoError(t, err) {
		assert.Equal(t, ctx, cx)
		assert.Equal(t, 1, rs.Runs())
		assert.Equal(t, 1, rs.Rollbacks())
	}
}
