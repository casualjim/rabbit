package steps

import (
	"context"

	"github.com/cenkalti/backoff"
	"github.com/hashicorp/errwrap"
)

// IsCanceled returns true when this error contains or is an error 
// that means execution was canceled
func IsCanceled(err error) bool {
	return errwrap.Contains(err, context.Canceled.Error()) ||
		errwrap.Contains(err, context.DeadlineExceeded.Error())
}

// StepErr creates a new error in a step
func StepErr(err error) *StepError {
	switch e := err.(type) {
	case *StepError:
		return e
	case *backoff.PermanentError:
		return StepErr(e.Err)
	case *PermanentError:
		return StepErr(e.Err)
	default:
		return &StepError{Err: err}
	}
}

// StepError contains the step name and the error message for the step
type StepError struct {
	Err error
}

func (s *StepError) Error() string {
	return s.Err.Error()
}

// WrappedErrors implements errwrap.Wrapper from https://github.com/hashicorp/errwrap
func (s *StepError) WrappedErrors() []error {
	return []error{s.Err}
}

// TransientErr creates a new recoverable error
func TransientErr(err error) *TransientError {
	switch e := err.(type) {
	case *StepError:
		return TransientErr(e.Err) //unwrap recursively
	case *TransientError:
		return e
	default:
		return &TransientError{Err: err}
	}
}

// TransientError will make not termintate the exeuction of a sequential step
type TransientError struct {
	Err error
}

func (s *TransientError) Error() string {
	return s.Err.Error()
}

// WrappedErrors implements errwrap.Wrapper from https://github.com/hashicorp/errwrap
func (s *TransientError) WrappedErrors() []error {
	return []error{s.Err}
}

// PermanentErr returns a permanent error for use in the retry policy as circuit breaker
func PermanentErr(err error) *PermanentError {
	switch e := err.(type) {
	case *backoff.PermanentError:
		return &PermanentError{Err: e.Err}
	case *PermanentError:
		return e
	default:
		return &PermanentError{Err: err}
	}
}

// PermanentError signals to the retry policy that the operation should not be retried.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return e.Err.Error()
}

// WrappedErrors implements errwrap.Wrapper from https://github.com/hashicorp/errwrap
func (e *PermanentError) WrappedErrors() []error {
	return []error{e.Err}
}
