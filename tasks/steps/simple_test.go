package steps_test

import (
	"context"
	"testing"

	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/stretchr/testify/require"
)
import "github.com/stretchr/testify/assert"

type tk string

func TestStepName(t *testing.T) {
	sn := steps.StepName("the name")
	assert.Equal(t, "the name", sn.Name())
}

func TestZeroStep(t *testing.T) {
	st := steps.Zero
	ctx, err := st.Run(nil)
	assert.NoError(t, err)
	assert.Nil(t, ctx)

	ctx, err = st.Rollback(nil)
	assert.NoError(t, err)
	assert.Nil(t, ctx)
}

func TestSimpleStep_Run(t *testing.T) {
	st := steps.Stateless(
		"simple-run-1",
		func(c context.Context) (context.Context, error) {
			return context.WithValue(c, tk("something"), "the value"), nil
		},
		noopFn,
	)

	c, e := steps.Plan(steps.Run(st)).Execute()
	if assert.NoError(t, e) {
		assert.Equal(t, "the value", c.Value(tk("something")))
	}

	c2, e2 := steps.Stateless("simple-run-2", nil, nil).Run(c)
	if assert.NoError(t, e2) {
		assert.Equal(t, c, c2)
	}
}

func TestSimpleStep_Rollback(t *testing.T) {
	st := steps.Stateless("simple-rb-1", failFn, noopFn)
	_, e := steps.Plan(steps.Run(st)).Execute()
	require.NoError(t, e)

	_, e2 := steps.Plan(steps.Run(steps.Stateless("simple-rb-2", failFn, nil))).Execute()
	assert.NoError(t, e2)
}

func TestAtomicStep_Run(t *testing.T) {
	st := steps.StatelessAtomic(
		"atomic-run-1",
		func(c context.Context) (context.Context, error) {
			return context.WithValue(c, tk("something"), "the value"), nil
		},
		noopFn,
	)

	c, e := steps.Plan(steps.Run(st)).Execute()
	if assert.NoError(t, e) {
		assert.Equal(t, "the value", c.Value(tk("something")))
	}

	c2, e2 := steps.StatelessAtomic("atomic-run-2", nil, nil).Run(c)
	if assert.NoError(t, e2) {
		assert.Equal(t, c, c2)
	}
}

func TestAtomicStep_Rollback(t *testing.T) {
	st := steps.StatelessAtomic("atomic-rb-1", failFn, noopFn)
	_, e := steps.Plan(steps.Run(st)).Execute()
	require.NoError(t, e)

	_, e2 := steps.Plan(steps.Run(steps.StatelessAtomic("atomic-rb-2", failFn, nil))).Execute()
	assert.NoError(t, e2)
}
