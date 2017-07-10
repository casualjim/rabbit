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

	assert.Panics(t, func() { steps.If(GoRight).Then(nil).Name() })

	right := &countingStep{StepName: steps.StepName("true")}
	left := &countingStep{StepName: steps.StepName("false")}

	assert.Equal(t, "~true", steps.If(GoRight).Then(right).Name())

	cond := steps.If(GoRight).Then(right).Else(left)
	ctx, err := steps.Plan(
		steps.Should(rollback.OnCancel),
		steps.Run(cond),
	).Execute()

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, "true|false", cond.Name())
		assert.Equal(t, 1, right.Runs())
		assert.Equal(t, 0, left.Runs())
	}

	rightFail := failRun("branching-fail")
	ctx, err = steps.Plan(steps.Run(
		steps.If(GoRight).Then(rightFail).Else(left),
	)).Execute()
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
	ctx, err = steps.Plan(steps.Run(
		steps.If(GoLeft).Then(right).Else(left),
	)).Execute()

	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 0, right.Runs())
		assert.Equal(t, 1, left.Runs())
	}

	rightFail = failRun("branching-fail-right")
	leftFail := failRun("branching-fail-left")
	ctx, err = steps.Plan(steps.Run(
		steps.If(GoLeft).Then(right).Else(leftFail),
	)).Execute()
	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 0, rightFail.Runs())
		assert.Equal(t, 1, leftFail.Runs())
		assert.Equal(t, 0, rightFail.Rollbacks())
		assert.Equal(t, 1, leftFail.Rollbacks())
	}
}

func TestBranchingEvents_SingleBranch(t *testing.T) {
	bus := testBus()

	step := stepRun("rightOnly", nil)
	plan := steps.Plan(
		steps.PublishTo(bus),
		steps.Run(steps.If(GoLeft).Then(step)),
	)

	ctx, err := plan.Execute()
	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 0, step.Runs())
		assert.Equal(t, 0, step.Rollbacks())
		bus.Assert(t, eventCounts{
			Registered: 2,
			RunSkipped: 1, // nothing to execute because we evaluate false
			RunRunning: 1, // only parent executed
			RunSuccess: 1, // only parent executed
		})
	}

	bus = testBus()
	step2 := stepRun("rightSide", nil)
	plan = steps.Plan(
		steps.PublishTo(bus),
		steps.Run(steps.If(GoRight).Then(step2)),
	)

	ctx, err = plan.Execute()
	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 1, step2.Runs())
		assert.Equal(t, 0, step2.Rollbacks())
		bus.Assert(t, eventCounts{
			Registered: 2,
			RunRunning: 2,
			RunSuccess: 2,
		})
	}

	bus = testBus()
	step3 := stepRun("rightSide", canceledFn)
	plan = steps.Plan(
		steps.PublishTo(bus),
		steps.Run(steps.If(GoRight).Then(step3)),
	)

	ctx, err = plan.Execute()
	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 1, step3.Runs())
		assert.Equal(t, 1, step3.Rollbacks())
		bus.Assert(t, eventCounts{
			Registered:  2,
			RunRunning:  2,
			RunCanceled: 2,
			RbRunning:   2,
			RbSuccess:   2,
		})
	}

	bus = testBus()
	step4 := failRun("rightSide")
	plan = steps.Plan(
		steps.PublishTo(bus),
		steps.Run(steps.If(GoRight).Then(step4)),
	)

	ctx, err = plan.Execute()
	if assert.NoError(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 1, step4.Runs())
		assert.Equal(t, 1, step4.Rollbacks())
		bus.Assert(t, eventCounts{
			Registered: 2,
			RunRunning: 2,
			RunFailed:  2,
			RbRunning:  2,
			RbSuccess:  2,
		})
	}

	bus = testBus()
	step5 := failRun("rightSide")
	step5.rollback = failFn
	plan = steps.Plan(
		steps.PublishTo(bus),
		steps.Run(steps.If(GoRight).Then(step5)),
	)

	ctx, err = plan.Execute()
	if assert.Error(t, err) {
		assert.NotNil(t, ctx)
		assert.Equal(t, 1, step5.Runs())
		assert.Equal(t, 1, step5.Rollbacks())
		bus.Assert(t, eventCounts{
			Registered: 2,
			RunRunning: 2,
			RunFailed:  2,
			RbRunning:  2,
			RbFailed:   2,
		})
	}
}
