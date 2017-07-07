package step_test

import (
	"context"
	"testing"

	"github.com/casualjim/rabbit/tasks/step"
	"github.com/stretchr/testify/require"
)
import "github.com/stretchr/testify/assert"

type tk string

func TestZeroStep(t *testing.T) {
	st := step.Zero
	ctx, err := st.Run(nil)
	assert.NoError(t, err)
	assert.Nil(t, ctx)

	ctx, err = st.Rollback(nil)
	assert.NoError(t, err)
	assert.Nil(t, ctx)
}

func TestSimpleStep_Run(t *testing.T) {
	st := step.Stateless(
		step.Run(func(c context.Context) (context.Context, error) {
			return context.WithValue(c, tk("something"), "the value"), nil
		}),
		step.Rollback(noopFn),
	)

	c, e := step.WithContext(context.Background()).Run(st)
	if assert.NoError(t, e) {
		assert.Equal(t, "the value", c.Value(tk("something")))
	}

	c2, e2 := step.Stateless(nil, nil).Run(c)
	if assert.NoError(t, e2) {
		assert.Equal(t, c, c2)
	}
}

func TestSimpleStep_Rollback(t *testing.T) {
	st := step.Stateless(failFn, noopFn)
	c, e := step.WithContext(context.Background()).Run(st)
	require.NoError(t, e)

	c2, e2 := step.WithContext(context.Background()).Run(step.Stateless(failFn, nil))
	if assert.NoError(t, e2) {
		assert.Equal(t, c, c2)
	}
}

func TestAtomicStep_Run(t *testing.T) {
	st := step.StatelessAtomic(
		step.Run(func(c context.Context) (context.Context, error) {
			return context.WithValue(c, tk("something"), "the value"), nil
		}),
		step.Rollback(noopFn),
	)

	c, e := step.WithContext(context.Background()).Run(st)
	if assert.NoError(t, e) {
		assert.Equal(t, "the value", c.Value(tk("something")))
	}

	c2, e2 := step.StatelessAtomic(nil, nil).Run(c)
	if assert.NoError(t, e2) {
		assert.Equal(t, c, c2)
	}
}

func TestAtomicStep_Rollback(t *testing.T) {
	st := step.StatelessAtomic(failFn, noopFn)
	c, e := step.WithContext(context.Background()).Run(st)
	require.NoError(t, e)

	c2, e2 := step.WithContext(context.Background()).Run(step.StatelessAtomic(failFn, nil))
	if assert.NoError(t, e2) {
		assert.Equal(t, c, c2)
	}
}
