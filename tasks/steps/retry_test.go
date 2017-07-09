package steps_test

import (
	"context"
	"testing"
	"time"

	"github.com/casualjim/rabbit/tasks/rollback"
	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/cenkalti/backoff"
	"github.com/stretchr/testify/assert"
)

func TestRetryMax_Fail(t *testing.T) {
	failStep := failRun("retry-max-fail")

	retry := steps.Retry(backoff.WithMaxTries(backoff.NewConstantBackOff(20*time.Millisecond), 3), failStep)
	assert.Equal(t, failStep.Name(), retry.Name())
	_, err := steps.Plan(steps.Run(retry)).Execute()
	if assert.NoError(t, err) {
		assert.Equal(t, 4, failStep.Runs())
		assert.Equal(t, 1, failStep.Rollbacks())
	}
}
func TestRetryMax_Success(t *testing.T) {
	var cnt int
	var expected context.Context
	successStep := stepRun("retry-max-success", func(c context.Context) (context.Context, error) {
		if cnt > 0 {
			expected = context.WithValue(c, tk("retrykey"), "blah")
			return expected, nil
		}
		cnt++
		return c, assert.AnError
	})

	retry := steps.Retry(backoff.WithMaxTries(backoff.NewConstantBackOff(20*time.Millisecond), 3), successStep)
	assert.Equal(t, successStep.Name(), retry.Name())
	ctx, err := steps.Plan(steps.Run(retry)).Execute()
	if assert.NoError(t, err) {
		assert.Equal(t, expected, ctx)
		assert.Equal(t, 2, successStep.Runs())
		assert.Equal(t, 0, successStep.Rollbacks())
	}
}

func TestRetryMax_CircuitBreaker(t *testing.T) {
	var cnt int
	breakingStep := stepRun("retry-max-break", func(c context.Context) (context.Context, error) {
		if cnt > 1 {
			return c, steps.PermanentErr(assert.AnError)
		}
		cnt++
		return c, assert.AnError
	})

	retry := steps.Retry(backoff.WithMaxTries(backoff.NewConstantBackOff(20*time.Millisecond), 8), breakingStep)
	_, err := steps.Plan(steps.Should(rollback.Never), steps.Run(retry)).Execute()
	if assert.Error(t, err) {
		assert.Equal(t, 3, breakingStep.Runs())
		assert.Equal(t, 0, breakingStep.Rollbacks())
	}
}

func TestRetryMax_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var cnt int
	breakingStep := stepRun("retry-max-cancel", func(c context.Context) (context.Context, error) {
		if cnt > 1 {
			cancel()
		}
		cnt++
		return c, assert.AnError
	})

	retry := steps.Retry(backoff.WithMaxTries(backoff.NewConstantBackOff(20*time.Millisecond), 8), breakingStep)

	_, err := steps.Plan(
		steps.ParentContext(ctx),
		steps.Should(rollback.Never),
		steps.Run(retry),
	).Execute()
	if assert.Error(t, err) {
		assert.Equal(t, 3, breakingStep.Runs())
		assert.Equal(t, 0, breakingStep.Rollbacks())
	}
}
