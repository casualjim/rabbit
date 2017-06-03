///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"

	"github.com/casualjim/rabbit/joint"
)

//StepInfo has the information of a step
type StepInfo struct {
	State State
	Name  string
}

//Step is one step of a Task, it can be one single operation or has sequential/parallel steps
type Step interface {
	Run(context.Context) (context.Context, *joint.Error)
	Rollback(context.Context) (context.Context, *joint.Error)
	GetInfo() StepInfo
	SetState(State)
}

//StepOpts includes the basic options of a step
type StepOpts struct {
	Name  string
	State State

	//The successFn signature takes the parent Step, the context and the child step
	//so that if any information from child step is needed, it can be done in this function.
	//Same reason for failFn.
	successFn func(Step, context.Context, Step)
	failFn    func(Step, context.Context, Step, *joint.Error)
}

//GenericStep is a generic Step
type GenericStep struct {
	StepOpts
	Steps []Step
}

func NewGenericStep(stepOpts StepOpts, steps ...Step) *GenericStep {
	return &GenericStep{
		StepOpts: stepOpts,
		Steps:    steps,
	}
}

///////////////////////////////////////////////////////////////////////
//Each specific step is supposed to have its own methods defined.
//Those methods on StepOpts are mostly dummy ones in case if a step does not have or
//does not need an implementation on certain methods.

func (s *GenericStep) Run(reqCtx context.Context) (context.Context, *joint.Error) {
	return reqCtx, nil
}

func (s *GenericStep) Rollback(reqCtx context.Context) (context.Context, *joint.Error) {
	return reqCtx, nil
}

func (s *GenericStep) GetInfo() StepInfo {
	return StepInfo{
		Name:  s.Name,
		State: s.State,
	}
}

func (s *GenericStep) SetState(state State) {
	s.State = state
}

///////////////////////////////////////////////////////////////////////
//Utility functions for Step

//GetDeepestActiveSteps returns the leaf steps on the tree of active steps, i.e. the deepest ones
func GetDeepestActiveSteps(step Step) []Step {
	info := step.GetInfo()
	if info.State != StateProcessing && info.State != StateRollingback {
		return nil
	}

	var res []Step

	switch s := step.(type) {
	case *SeqStep:
		if s.Steps == nil {
			res = append(res, step)
			return res
		}
		for _, thisStep := range s.Steps {
			info := thisStep.GetInfo()
			if info.State == StateProcessing || info.State == StateRollingback {
				return GetDeepestActiveSteps(thisStep)
			}
		}

		res = append(res, step)
		return res

	default:
		res = append(res, s)
		return res
	}
}

//GetFirstLevelActiveSteps returns the first level steps on the tree of active steps
func GetFirstLevelActiveSteps(step Step) []Step {
	info := step.GetInfo()
	if info.State != StateProcessing && info.State != StateRollingback {
		return nil
	}

	var res []Step
	switch s := step.(type) {
	case *SeqStep:
		if s.Steps == nil {
			res = append(res, step)
			return res
		}
		for _, thisStep := range s.Steps {
			info := thisStep.GetInfo()
			if info.State == StateProcessing || info.State == StateRollingback {
				res = append(res, thisStep)
			}
		}
		return res
	default:
		res = append(res, step)
		return res
	}
}
