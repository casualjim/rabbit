///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"errors"
	"sync"

	"github.com/casualjim/rabbit"
	"github.com/casualjim/rabbit/eventbus"
)

type ParallelStep struct {
	GenericStep
}

//NewParallelStep creates a new parallel step whose substeps can be executed at the same time
//Note that the new Step should be of state StateWaiting, and all of its substeps should be of state StateWaiting too.
func NewParallelStep(stepInfo StepInfo,
	log rabbit.Logger,
	contextfn func([]context.Context) context.Context,
	errorfn func([]error) error,
	handlerFn func(eventbus.Event) error,
	steps ...Step) *ParallelStep {
	//the caller is responsible to make sure stepOpts and all step's state are set to StateWaiting

	s := &ParallelStep{
		GenericStep: GenericStep{
			info:           stepInfo,
			Log:            Logger(log),
			contextHandler: NewContextHandler(contextfn),
			errorHandler:   NewErrorHandler(errorfn),
			eventHandler:   NewEventHandler(handlerFn),
			Steps:          steps},
	}

	for _, step := range steps {
		step.SetLogger(s.Log)
	}
	return s
}

func (s *ParallelStep) Run(reqCtx context.Context, bus eventbus.EventBus) (context.Context, error) {
	s.SetState(StateProcessing)

	bus.Subscribe(s.eventHandler)

	var runError error
	var resultCtx context.Context
	var resultErr error
	var cancelErr error
	ctxc := make(chan context.Context)
	errc := make(chan error)
	done := make(chan struct{})

	var wgService sync.WaitGroup

	// Start a background goroutine for handling substep passed ctxs
	wgService.Add(1)
	go func(reqCtx context.Context) {
		getCtx := false

		ctxs := []context.Context{reqCtx}
		for r := range ctxc {
			ctxs = append(ctxs, r)
			getCtx = true
		}
		if getCtx {
			resultCtx = s.contextHandler(ctxs)
		}
		wgService.Done()
	}(reqCtx)

	// Start a background goroutine for handling substep passed errors
	wgService.Add(1)
	go func() {
		var stepErrors []error

		for e := range errc {
			stepErrors = append(stepErrors, e)
		}
		if stepErrors != nil {
			resultErr = s.errorHandler(stepErrors)
		}
		wgService.Done()
	}()

	var wgSubSteps sync.WaitGroup

OuterLoop:
	for _, step := range s.Steps {
		ctx := reqCtx
		// Before we run every step, check whether root step is canceled already
		select {
		case <-ctx.Done():
			cancelErr = errors.New("step " + s.GetName() + " canceled")
			s.Log.Printf("step %s got canceled", s.GetName())
			break OuterLoop
		default:
		}
		wgSubSteps.Add(1)
		go func(step Step, ctx context.Context) {
			ctx, err := step.Run(ctx, bus)
			if err != nil {
				errc <- err
			} else {
				ctxc <- ctx
			}
			wgSubSteps.Done()
		}(step, ctx)
	}

	// If not canceled while running substeps, start a background goroutine to
	// capture cancelErr
	if cancelErr == nil {
		wgService.Add(1)
		go func() {
			select {
			case <-reqCtx.Done():
				cancelErr = errors.New("step " + s.GetName() + " canceled")
				s.Log.Printf("step %s got canceled", s.GetName())
			case <-done:
				wgService.Done()
			}
		}()
	}

	wgSubSteps.Wait()

	// Note: after wgSubSteps.Wait() and before we close done, if reqCtx were
	// associated with a timer(cancle and timeout ctx) and it fired, there are
	// still chances that cancelErr is set even though wgSubSteps.Wait()'s already
	// returned here, meaning whole step is done before cancelling.
	// TODO: So we still have a bug here
	close(done)

	// After wgSubSteps.Wait() returned, all workers have returned, so it's safe to
	// close ctxc and errc here. Thus goroutines which are ranging on these
	// channels will safely return
	close(ctxc)
	close(errc)

	wgService.Wait()

	// No need to put a lock on cancelErr here, because wgService.Wait()
	// guarantees cancelErr is written before this read
	ce := cancelErr

	var errs []error
	if ce != nil {
		if resultErr != nil {
			errs = append(errs, resultErr)
		}
		errs = append(errs, cancelErr)

		_, rollbackError := s.Rollback(reqCtx, bus)

		if rollbackError != nil {
			errs = append(errs, rollbackError)
		}
		runError = s.errorHandler(errs)

		s.Log.Printf("step %s canceled. %s", s.GetName(), runError)
		return reqCtx, runError

	} else if resultErr != nil {
		errs = append(errs, resultErr)
		runError = s.errorHandler(errs)
		s.Fail(reqCtx, runError)

		s.Log.Printf("step %s failed, %s", s.GetName(), runError.Error())
		return reqCtx, runError

	} else if resultCtx != nil {
		s.Success(resultCtx)
		return resultCtx, nil
	}

	return reqCtx, nil
}
