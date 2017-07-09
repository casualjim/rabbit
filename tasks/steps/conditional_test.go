package steps_test

import (
	"context"
	"testing"

	"github.com/casualjim/rabbit/tasks/rollback"
	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/stretchr/testify/assert"
)

func GoRight(_ context.Context) bool { return true }
func GoLeft(_ context.Context) bool  { return false }

func TestBranching(t *testing.T) {
	right := &countingStep{}
	left := &countingStep{}

	cond := steps.If(GoRight).Then(right).Else(left)
	ctx, err := steps.Execution(steps.Should(rollback.OnCancel)).Run(cond)

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Empty(t, cond.Name())
		assert.Equal(t, 1, right.Runs())
		assert.Equal(t, 0, left.Runs())
	}

	rightFail := failRun("branching-fail")
	ctx, err = steps.Execution().Run(
		steps.If(GoRight).Then(rightFail).Else(left),
	)
	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 1, rightFail.Runs())
		assert.Equal(t, 0, left.Runs())
		assert.Equal(t, 1, rightFail.Rollbacks())
		assert.Equal(t, 0, left.Rollbacks())
	}

	right = &countingStep{
		StepName: steps.StepName("right"),
	}
	left = &countingStep{
		StepName: steps.StepName("left"),
	}
	ctx, err = steps.Execution().Run(
		steps.If(GoLeft).Then(right).Else(left),
	)

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 0, right.Runs())
		assert.Equal(t, 1, left.Runs())
	}

	rightFail = failRun("branching-fail-right")
	leftFail := failRun("branching-fail-left")
	ctx, err = steps.Execution().Run(
		steps.If(GoLeft).Then(right).Else(leftFail),
	)
	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 0, rightFail.Runs())
		assert.Equal(t, 1, leftFail.Runs())
		assert.Equal(t, 0, rightFail.Rollbacks())
		assert.Equal(t, 1, leftFail.Rollbacks())
	}
}
