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

	ctx, err := steps.WithContext(context.Background()).Should(rollback.OnCancel).Run(
		steps.If(GoRight).Then(right).Else(left),
	)

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 1, right.Runs())
		assert.Equal(t, 0, left.Runs())
	}

	rightFail := failRun()
	ctx, err = steps.WithContext(context.Background()).Run(
		steps.If(GoRight).Then(rightFail).Else(left),
	)
	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 1, rightFail.Runs())
		assert.Equal(t, 0, left.Runs())
		assert.Equal(t, 1, rightFail.Rollbacks())
		assert.Equal(t, 0, left.Rollbacks())
	}

	right = &countingStep{}
	left = &countingStep{}
	ctx, err = steps.WithContext(context.Background()).Run(
		steps.If(GoLeft).Then(right).Else(left),
	)

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 0, right.Runs())
		assert.Equal(t, 1, left.Runs())
	}

	rightFail = failRun()
	leftFail := failRun()
	ctx, err = steps.WithContext(context.Background()).Run(
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
