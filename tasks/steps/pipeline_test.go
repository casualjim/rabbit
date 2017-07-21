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

	ctx, err := steps.Plan(steps.Run(
		steps.Pipeline(
			"pipeline-1",
			step,
			step,
			step,
			step,
		),
	)).Execute()

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 4, step.Runs())
	}

	step = &countingStep{}
	stepFail := failRun("pipeline-fail")
	ctx, err = steps.Plan(steps.Run(
		steps.Pipeline(
			"pipeline-2",
			step,
			step,
			stepFail,
			stepFail,
		),
	)).Execute()

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 2, step.Runs())
		assert.Equal(t, 1, stepFail.Runs())
		assert.Equal(t, 2, step.Rollbacks())
		assert.Equal(t, 1, stepFail.Rollbacks())
	}
}

func TestPipeline_TransientErr(t *testing.T) {
	step := &countingStep{}
	var cnt int

	stepFail := stepRun("pipeline-fail", func(c context.Context) (context.Context, error) {
		if cnt == 0 {
			cnt += 1
			return c, steps.TransientErr(assert.AnError)
		}
		return c, assert.AnError
	})

	ctx, err := steps.Plan(steps.Run(
		steps.Pipeline(
			"pipeline-transient",
			step,
			step,
			stepFail,
			stepFail,
			stepFail,
			step,
		),
	)).Execute()

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 2, step.Runs())
		assert.Equal(t, 2, stepFail.Runs())
		assert.Equal(t, 2, step.Rollbacks())
		assert.Equal(t, 2, stepFail.Rollbacks())
	}
}

func TestPipeline_Canceled(t *testing.T) {
	step := &countingStep{StepName: steps.StepName("pipeline-canceled-1")}

	exec := steps.Plan(steps.Run(
		steps.Pipeline(
			"pipeline-cancel-1",
			step,
			step,
			step,
			step,
		)),
	)
	ctx := exec.Context()
	exec.Cancel()
	ctx2, err := exec.Execute()

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx2)
		assert.Equal(t, ctx, ctx2)
		assert.Equal(t, 0, step.Runs())
	}

	ctxt, cancel := context.WithCancel(context.Background())
	runStep := &countingStep{StepName: steps.StepName("pipeline-canceled-run-1")}
	cancelStep := &countingStep{
		StepName: steps.StepName("pipeline-canceled-cancel-1"),
		run: func(c context.Context) (context.Context, error) {
			cancel()
			return c, nil
		},
	}
	rbFailStep := &countingStep{
		StepName: steps.StepName("pipeline-canceled-rollback-1"),
		rollback: func(c context.Context) (context.Context, error) {
			return c, errors.New("expected")
		},
	}

	exec2 := steps.Plan(
		steps.ParentContext(ctxt),
		steps.Run(steps.Pipeline(
			"pipeline-cancel-2",
			runStep,
			rbFailStep,
			cancelStep,
			runStep,
		)),
	)

	ctx4, err2 := exec2.Execute()

	if assert.NoError(t, err2) {
		assert.NotNil(t, ctx4)
		assert.Equal(t, 1, runStep.Runs())
		assert.Equal(t, 1, rbFailStep.Runs())
		assert.Equal(t, 1, cancelStep.Runs())
		assert.Equal(t, 1, runStep.Rollbacks())
		assert.Equal(t, 1, rbFailStep.Rollbacks())
		assert.Equal(t, 1, cancelStep.Rollbacks())
	}

	runst := stepRun("cancel-run-2", nil)
	cancelst := stepRun("cancel-cancel-2", canceledFn)
	steps.Plan(
		steps.Run(steps.Pipeline(
			"pipeline-cancel-3",
			runst,
			cancelst,
		)),
	).Execute()
	assert.Equal(t, 1, runst.Runs())
	assert.Equal(t, 1, cancelst.Runs())
	assert.Equal(t, 1, runStep.Rollbacks())
	assert.Equal(t, 1, rbFailStep.Rollbacks())
}

func TestPipeline_Events(t *testing.T) {

}
