package steps_test

import (
	"testing"

	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/cenkalti/backoff"
	"github.com/stretchr/testify/assert"
)

func TestStepError(t *testing.T) {
	t.Parallel()

	err := assert.AnError
	se := steps.StepErr(err)
	assert.Equal(t, err, se.Err)
	assert.EqualError(t, err, se.Error())
	assert.Equal(t, []error{se.Err}, se.WrappedErrors())

	se2 := steps.StepErr(se)
	assert.Equal(t, se, se2)

	be := steps.PermanentErr(err)
	assert.Equal(t, se, steps.StepErr(be))

	be2 := backoff.Permanent(err)
	assert.Equal(t, se, steps.StepErr(be2))
}

func TestPermanentErr(t *testing.T) {
	pe := steps.PermanentErr(assert.AnError)
	assert.Equal(t, assert.AnError, pe.Err)
	assert.EqualError(t, assert.AnError, pe.Error())
	assert.Equal(t, []error{assert.AnError}, pe.WrappedErrors())

	pe2 := steps.PermanentErr(pe)
	assert.Equal(t, pe, pe2)

	be := backoff.Permanent(assert.AnError)
	assert.Equal(t, &steps.PermanentError{Err: be.Err}, steps.PermanentErr(be))
}

func TestTransientError(t *testing.T) {
	t.Parallel()

	err := assert.AnError
	se := steps.TransientErr(err)
	assert.Equal(t, err, se.Err)
	assert.EqualError(t, err, se.Error())
	assert.Equal(t, []error{se.Err}, se.WrappedErrors())

	se2 := steps.TransientErr(se)
	assert.Equal(t, se, se2)

	se3 := steps.StepErr(err)
	se4 := steps.TransientErr(se3)
	assert.Equal(t, se3.Err, se4.Err)
}
