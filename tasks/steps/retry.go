package steps

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
)

// Retry the step with the specified policy
func Retry(policy backoff.BackOff, step Step) Step {
	return &retryStep{
		policy: policy,
		step:   step,
	}
}

type retryStep struct {
	policy backoff.BackOff
	step   Step
}

func (r *retryStep) Name() string {
	return r.step.Name()
}

func (r *retryStep) Run(ctx context.Context) (context.Context, error) {
	policy := backoff.WithContext(r.policy, ctx)
	notifier := func(e error, next time.Duration) {}

	fctx := ctx
	op := func() error {
		cx, err := r.step.Run(ctx)
		if err == nil {
			fctx = cx
		} else {
			if e, ok := err.(*PermanentError); ok {
				return backoff.Permanent(e.Err)
			}
		}

		return err
	}

	err := backoff.RetryNotify(op, policy, notifier)
	return fctx, err
}

func (r *retryStep) Rollback(ctx context.Context) (context.Context, error) {
	return r.step.Rollback(ctx)
}
