package steps

import (
	"context"
	"sync"

	multierror "github.com/hashicorp/go-multierror"
)

// Pipeline executes the steps sequentially and the result of each step is passed into the next step
func Pipeline(name StepName, steps ...Step) Step {
	return &pipelineStep{steps: steps, idx: -1, StepName: name}
}

type pipelineStep struct {
	StepName
	steps []Step
	idx   int
	m     sync.Mutex
}

func (s *pipelineStep) Announce(ctx context.Context, callback func(string)) {
	s.StepName.Announce(ctx, callback)
	pt := SetParentName(ctx, s.Name())
	for _, step := range s.steps {
		step.Announce(pt, callback)
	}
}

func (s *pipelineStep) Run(ct context.Context) (context.Context, error) {
	s.m.Lock()
	defer s.m.Unlock()

	select {
	case <-ct.Done():
		//we're done bail
		return ct, ct.Err()
	default:
	}

	ctx := SetParentName(ct, s.Name())
	var err error
	for i, step := range s.steps {
		s.idx = i // record we started, step at index
		var ie error
		PublishRunEvent(ctx, step.Name(), StateProcessing)
		ctx, ie = step.Run(ctx)
		ctx = OverrideParentName(ctx, GetParentName(ct), s.Name())
		if ie != nil {
			if _, ok := ie.(*TransientError); ok {
				PublishRunEvent(ctx, step.Name(), StateFailed)
				continue
			}
			if IsCanceled(ie) {
				PublishRunEvent(ctx, step.Name(), StateCanceled)
			} else {
				PublishRunEvent(ctx, step.Name(), StateFailed)
			}
			err = multierror.Append(err, ie)
			break
		} else {
			PublishRunEvent(ctx, step.Name(), StateSuccess)
		}
		select {
		case <-ctx.Done():
			//we're done, bail
			return ctx, multierror.Append(err, ctx.Err())
		default:
		}
	}
	return ctx, err
}

func (s *pipelineStep) Rollback(ct context.Context) (context.Context, error) {
	s.m.Lock()
	defer s.m.Unlock()

	if s.idx < 0 {
		PublishRollbackEvent(ct, s.Name(), StateSkipped)
		return ct, nil
	}

	ctx := SetParentName(ct, s.Name())
	var err error
	for i := s.idx + 1; i < len(s.steps); i++ { // maybe this should also be reverse order
		step := s.steps[i]
		PublishRollbackEvent(ctx, step.Name(), StateSkipped)
	}
	for i := s.idx; i >= 0; i-- {
		step := s.steps[i]
		PublishRollbackEvent(ctx, step.Name(), StateProcessing)
		ctx, err = step.Rollback(ctx)
		if err != nil {
			PublishRollbackEvent(ctx, step.Name(), StateFailed)
			continue
		}
		PublishRollbackEvent(ctx, step.Name(), StateSuccess)
	}
	return ctx, nil
}
