///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"errors"
	"sync"
)

type ParalStep struct {
	GenericStep
	aggContextFn func(context.Context, context.Context) context.Context
	aggErrorFn   func([]error) error
}

//NewParalStep creates a new parallel step whose substeps can be executed at the same time
//Note that the new Step should be of state StepStateNone, and all of its substeps should be of state StepStateNone too.
func NewParalStep(stepInfo StepInfo, contextfn func(context.Context, context.Context) context.Context,
	errorfn func([]error) error, steps ...Step) *ParalStep {
	//the caller is responsible to make sure stepOpts and all step's state are set to StateNone
	return &ParalStep{
		GenericStep:  GenericStep{StepInfo: stepInfo, Steps: steps},
		aggContextFn: contextfn,
		aggErrorFn:   errorfn,
	}
}

func (s *ParalStep) Success(reqCtx context.Context) {
	if s.successFn == nil {
		s.SetState(StateCompleted)
	} else {
		s.successFn(reqCtx, s)
	}
}

func (s *ParalStep) Fail(reqCtx context.Context, err error) {
	if s.failFn == nil {
		s.SetState(StateFailed)
	} else {
		s.failFn(reqCtx, s, err)
	}
}

func (s *ParalStep) Run(reqCtx context.Context) (context.Context, error) {

	s.State = StateProcessing
	var runError error

	ctxc := make(chan context.Context)
	errc := make(chan error)

	var resultCtx context.Context
	var resultErr error
	var cancelErr error

	var wgCtx sync.WaitGroup
	wgCtx.Add(1)

	go func(reqCtx context.Context) {
		ctx := reqCtx
		getCtx := false

		for r := range ctxc {
			ctx = s.aggContextFn(ctx, r)
			getCtx = true
		}
		if getCtx {
			resultCtx = ctx
		}
		wgCtx.Done()
	}(reqCtx)

	var wgErr sync.WaitGroup
	wgErr.Add(1)
	go func() {
		var stepErrors []error

		for e := range errc {
			stepErrors = append(stepErrors, e)
		}
		if stepErrors != nil {
			resultErr = s.aggErrorFn(stepErrors)
		}
		wgErr.Done()
	}()

	go func(ctx context.Context) {
		select {
		case <-reqCtx.Done():
			cancelErr = errors.New("step " + s.Name + " canceled")
		}

	}(reqCtx)

	var wg1 sync.WaitGroup
	wg1.Add(len(s.Steps))
	for _, step := range s.Steps {
		ctx := reqCtx
		go func(step Step, ctx context.Context) {
			//for parallel step, we do not care about returned ctx from step.Run()

			ctx, err := step.Run(ctx)
			if err != nil {
				//what do we do? let the consumer decide I guess
				//do we still need the context from this run aggregated? - no

				errc <- err
			} else {
				ctxc <- ctx
			}
			wg1.Done()
		}(step, ctx)
	}

	wg1.Wait()

	//the problem of this logic is if we always wait , then cancel signal is not canceling anything?
	//even the example in https://blog.golang.org/pipelines is not actually canceling anything.
	//it just not reporting to the result if canceld.
	//it seems the only use is that we get the cancel signal, we rollback after wait finishes

	close(ctxc)
	close(errc)

	wgCtx.Wait()
	wgErr.Wait()

	var errs []error

	// fmt.Printf("step %s processing result\n", s.Name)

	if cancelErr != nil {
		errs = append(errs, cancelErr)

		if resultErr != nil {
			errs = append(errs, resultErr)
		}
		_, rollbackError := s.Rollback(reqCtx)

		if rollbackError != nil {
			errs = append(errs, rollbackError)
		}
		runError = s.aggErrorFn(errs)
		return reqCtx, runError
	} else if resultErr != nil {
		errs = append(errs, resultErr)
		runError = s.aggErrorFn(errs)
		s.Fail(reqCtx, runError)

		return reqCtx, runError
	} else if resultCtx != nil {
		s.Success(resultCtx)
		return resultCtx, nil
	}

	return reqCtx, nil
}
