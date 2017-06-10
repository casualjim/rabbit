///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"errors"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/casualjim/rabbit/eventbus"
)

type SeqStep struct {
	GenericStep
}

//NewSeqStep creates a new sequential step
//Note that the new SeqStep should be of state StepStateNone, and all of its substeps should be of state StepStateNone too.
func NewSeqStep(stepInfo StepInfo,
	log logrus.FieldLogger,
	contextfn func([]context.Context) context.Context,
	errorfn func([]error) error,
	handlerFn func(eventbus.Event) error,
	steps ...Step) *SeqStep {
	//the caller is responsible to make sure stepOpts and all substep's state are set to StateNone

	s := &SeqStep{
		GenericStep: GenericStep{
			StepInfo:       stepInfo,
			log:            Logger(log),
			contextHandler: NewContextHandler(contextfn),
			errorHandler:   NewErrorHandler(errorfn),
			eventHandler:   NewEventHandler(handlerFn),
			Steps:          steps},
	}

	for _, step := range steps {
		step.SetLogger(s.log)
	}
	return s

}

//Run runs all the steps sequentially. The substeps are responsible to update their states.
func (s *SeqStep) Run(reqCtx context.Context, bus eventbus.EventBus) (context.Context, error) {
	s.State = StateProcessing

	if s.eventHandler != nil {
		bus.Subscribe(s.eventHandler)
	}

	var err error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for _, step := range s.Steps {
			//the step run is responsible for updating the ctx.Value(testRunTime)
			reqCtx, err = step.Run(reqCtx, bus)
			select {
			case <-reqCtx.Done():
				s.log.Debugf("step %s got canceled", s.Name)
				cancelErr := errors.New("step " + s.Name + " canceled")
				_, rollbackErr := s.Rollback(reqCtx, bus)
				err = s.errorHandler([]error{err, cancelErr, rollbackErr})
				wg.Done()
				return
			default:
			}
			if err != nil {
				break
			}
		}
		wg.Done()
	}()
	wg.Wait()
	if err != nil {
		s.Fail(reqCtx, err)
		s.log.Debugf("step %s failed, %s", s.Name, err.Error())
		return reqCtx, err
	}
	s.Success(reqCtx)
	return reqCtx, nil
}

func (s *SeqStep) Success(reqCtx context.Context) {
	if s.successFn == nil {
		s.SetState(StateCompleted)
	} else {
		s.successFn(reqCtx, s)
	}
}

func (s *SeqStep) Fail(reqCtx context.Context, err error) {
	if s.failFn == nil {
		s.SetState(StateFailed)
	} else {
		s.failFn(reqCtx, s, err)
	}
}
