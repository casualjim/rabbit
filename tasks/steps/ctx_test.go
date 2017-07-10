package steps_test

import (
	"context"
	"testing"

	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/stretchr/testify/assert"
)

func TestStepPath(t *testing.T) {
	ctx := context.Background()
	assert.Empty(t, steps.StepPath(ctx, ""))

	assert.Equal(t, "local", steps.StepPath(ctx, "local"))

	ctx = steps.SetParentName(ctx, "parent")
	assert.Equal(t, "parent", steps.StepPath(ctx, ""))

	assert.Equal(t, "parent.local", steps.StepPath(ctx, "local"))
}

func TestContextParentName(t *testing.T) {
	ctx := steps.SetParentName(context.Background(), "the step")
	assert.Equal(t, "the step", steps.GetParentName(ctx))

	ctx = context.WithValue(context.Background(), tk("dummy"), "blah")
	ctx2 := steps.SetParentName(ctx, "")
	assert.Equal(t, ctx, ctx2)
	assert.Empty(t, steps.GetParentName(ctx2))

	ctx3 := steps.SetParentName(steps.SetParentName(ctx, "parent"), "child")
	assert.NotEqual(t, ctx, ctx3)
	assert.Equal(t, "parent.child", steps.GetParentName(ctx3))
}

func TestContextStepName(t *testing.T) {
	ctx := steps.SetStepName(context.Background(), "the step")
	assert.Equal(t, "the step", steps.GetStepName(ctx))

	ctx = context.WithValue(context.Background(), tk("dummy"), "blah")
	ctx2 := steps.SetStepName(ctx, "")
	assert.Equal(t, ctx, ctx2)

	assert.Empty(t, steps.GetStepName(ctx2))
}

func TestContextTaskName(t *testing.T) {
	ctx := steps.SetTaskName(context.Background(), "the task")
	assert.Equal(t, "the task", steps.GetTaskName(ctx))

	ctx = context.WithValue(context.Background(), tk("dummy"), "blah")
	ctx2 := steps.SetTaskName(ctx, "")
	assert.Equal(t, ctx, ctx2)

	assert.Empty(t, steps.GetTaskName(ctx2))
}
