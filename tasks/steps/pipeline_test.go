package steps_test

import (
	"context"
	"errors"
	"testing"

	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/stretchr/testify/assert"
)

func TestPipeline(t *testing.T) {
	step := &countingStep{}

	ctx, err := steps.WithContext(context.Background()).Run(
		steps.Pipeline(
			step,
			step,
			step,
			step,
		),
	)

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 4, step.Runs())
	}

	step = &countingStep{}
	stepFail := failRun()
	ctx, err = steps.WithContext(context.Background()).Run(
		steps.Pipeline(
			step,
			step,
			stepFail,
			stepFail,
		),
	)

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 2, step.Runs())
		assert.Equal(t, 1, stepFail.Runs())
		assert.Equal(t, 2, step.Rollbacks())
		assert.Equal(t, 1, stepFail.Rollbacks())
	}
}

func TestPipeline_Cancelled(t *testing.T) {
	step := &countingStep{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ctx2, err := steps.WithContext(ctx).Run(
		steps.Pipeline(
			step,
			step,
			step,
			step,
		),
	)

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx2)
		assert.Equal(t, ctx, ctx2)
		assert.Equal(t, 0, step.Runs())
	}

	ctx3, cancel2 := context.WithCancel(context.Background())
	runStep := &countingStep{}
	cancelStep := &countingStep{
		run: func(c context.Context) (context.Context, error) {
			cancel2()
			return c, nil
		},
	}
	rbFailStep := &countingStep{
		rollback: func(c context.Context) (context.Context, error) {
			return c, errors.New("expected")
		},
	}

	ctx4, err2 := steps.WithContext(ctx3).Run(
		steps.Pipeline(
			runStep,
			rbFailStep,
			cancelStep,
			runStep,
		),
	)

	if assert.NoError(t, err2) {
		assert.NotNil(t, ctx4)
		assert.Equal(t, ctx3, ctx4)
		assert.Equal(t, 1, runStep.Runs())
		assert.Equal(t, 1, rbFailStep.Runs())
		assert.Equal(t, 1, cancelStep.Runs())
		assert.Equal(t, 1, runStep.Rollbacks())
		assert.Equal(t, 1, rbFailStep.Rollbacks())
		assert.Equal(t, 1, cancelStep.Rollbacks())
	}
}
