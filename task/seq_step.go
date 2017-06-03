///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"

	"github.com/casualjim/rabbit/joint"
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
func (s *SeqStep) Run(reqCtx context.Context) (context.Context, *joint.Error) {
	s.State = StateProcessing
	var e *joint.Error
	var lastStep Step
	for _, step := range s.Steps {
		//step.Run() is responsible for updating the ctx.Value(taskRunTime)
		reqCtx, e = step.Run(reqCtx)
		lastStep = step
		if e != nil {
			s.Fail(reqCtx, step, e)
			return reqCtx, e
		}
	}
	s.Success(reqCtx, lastStep)

	return reqCtx, nil
}

func (s *SeqStep) Success(reqCtx context.Context, step Step) {
	if s.successFn == nil {
		s.SetState(StateCompleted)
	} else {
		s.successFn(s, reqCtx, s)
	}
}

func (s *SeqStep) Fail(reqCtx context.Context, step Step, e *joint.Error) {
	if s.failFn == nil {
		s.SetState(StateFailed)
	} else {
		s.failFn(s, reqCtx, step, e)
	}
}
