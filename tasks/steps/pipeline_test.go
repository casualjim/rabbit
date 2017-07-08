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
			"pipeline-1",
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
	stepFail := failRun("pipeline-fail")
	ctx, err = steps.WithContext(context.Background()).Run(
		steps.Pipeline(
			"pipeline-2",
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
	step := &countingStep{StepName: steps.StepName("pipeline-cancelled-1")}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ctx2, err := steps.WithContext(ctx).Run(
		steps.Pipeline(
			"pipeline-cancel-1",
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
	runStep := &countingStep{StepName: steps.StepName("pipeline-cancelled-run-1")}
	cancelStep := &countingStep{
		StepName: steps.StepName("pipeline-cancelled-cancel-1"),
		run: func(c context.Context) (context.Context, error) {
			cancel2()
			return c, nil
		},
	}
	rbFailStep := &countingStep{
		StepName: steps.StepName("pipeline-cancelled-rollback-1"),
		rollback: func(c context.Context) (context.Context, error) {
			return c, errors.New("expected")
		},
	}

	ctx4, err2 := steps.WithContext(ctx3).Run(
		steps.Pipeline(
			"pipeline-cancel-2",
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
