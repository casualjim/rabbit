///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"errors"

	"github.com/casualjim/rabbit"
	"github.com/casualjim/rabbit/eventbus"
	"github.com/casualjim/rabbit/workerGroup"
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

	wkg := workerGroup.NewWorkerGroup()
	ctxc := wkg.MakeSink() // chan context used by substeps
	errc := wkg.MakeSink() // chan error used by substeps

	var cancelErr error

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
		wkg.Add(1)
		go func(step Step, ctx context.Context) {
			ctx, err := step.Run(ctx, bus)
			if err != nil {
				errc <- err
			} else {
				ctxc <- ctx
			}
			wkg.Done()
		}(step, ctx)
	}

	wkg.Wait()

	// Handling contexts passed by substeps
	var resultCtx context.Context

	ctxs := []context.Context{reqCtx}
	ctxsTmp := wkg.Fetch(ctxc)
	for _, ctx := range ctxsTmp {
		if c, ok := ctx.(context.Context); ok {
			ctxs = append(ctxs, c)
		}
	}
	if len(ctxs) > 1 {
		resultCtx = s.contextHandler(ctxs)
	}

	// Handling errors passed by substeps
	var runError error
	var resultErr error

	var stepErrors []error
	stepErrorsTmp := wkg.Fetch(errc)
	for _, stepError := range stepErrorsTmp {
		if e, ok := stepError.(error); ok {
			stepErrors = append(stepErrors, e)
		}
	}
	if stepErrors != nil {
		resultErr = s.errorHandler(stepErrors)
	}

	var errs []error
	if cancelErr != nil {
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
