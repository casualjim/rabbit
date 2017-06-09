///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"sync"
)

type SeqStep struct {
	GenericStep
}

//NewSeqStep creates a new sequential step
//Note that the new SeqStep should be of state StepStateNone, and all of its substeps should be of state StepStateNone too.
func NewSeqStep(stepInfo StepInfo, steps ...Step) *SeqStep {
	//the caller is responsible to make sure stepOpts and all step's state are set to StateNone
	return &SeqStep{GenericStep{StepInfo: stepInfo, Steps: steps}}
}

//Run runs all the steps sequentially. The substeps are responsible to update their states.
func (s *SeqStep) Run(reqCtx context.Context) (context.Context, error) {
	s.State = StateProcessing
	var err error

	//need to update to handle cancel
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for _, step := range s.Steps {
			//step.Run() is responsible for updating the ctx.Value(testRunTime)
			reqCtx, err = step.Run(reqCtx)
			select {
			case <-reqCtx.Done():
				//need to log the cancel
				reqCtx, err = s.Rollback(reqCtx)
				wg.Done()
				return
			default:
			}

		}
		wg.Done()
	}()
	wg.Wait()
	if err != nil {
		s.Fail(reqCtx, err)
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
