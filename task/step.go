///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"

	"github.com/casualjim/rabbit"
	"github.com/casualjim/rabbit/eventbus"
)

//StepInfo has the information of a step
type StepInfo struct {
	State State
	Name  string
}

//Step is one step of a Task, it can be one single operation or has sequential/parallel steps
type Step interface {
	//most likely only sequential step would need the returned context to pass to the next context
	Run(context.Context, eventbus.EventBus) (context.Context, error)
	Rollback(context.Context, eventbus.EventBus) (context.Context, error)
	GetInfo() StepInfo
	GetSteps() []Step
	SetLogger(rabbit.Logger)
	SetState(State)
}

//GenericStep is a generic Step
type GenericStep struct {
	StepInfo
	Steps []Step
	Log   rabbit.Logger

	//contextHandler handles the returned contexts from substeps, for example combines some values carried in those contexts
	contextHandler func([]context.Context) context.Context
	//errorhandler handles the returned errors from substeps. It can be designed to do pattern match and return another error,
	//or just put all the errors together (as the default error handler does)
	errorHandler func([]error) error
	//eventHandler handles events received from event bus.
	eventHandler eventbus.EventHandler

	successFn func(context.Context, Step)
	failFn    func(context.Context, Step, error)
}

func NewStepInfo(name string) StepInfo {
	return StepInfo{Name: name, State: StateNone}
}

func NewGenericStep(stepInfo StepInfo, log rabbit.Logger, steps ...Step) *GenericStep {
	return &GenericStep{
		StepInfo: stepInfo,
		Log:      Logger(log),
		Steps:    steps,
	}
}

///////////////////////////////////////////////////////////////////////
//Each specific step is supposed to have its own methods defined.
//Those methods on GenericStep are just here in case if a step does not have or
//does not need an implementation on certain methods.

func (s *GenericStep) Run(reqCtx context.Context, bus eventbus.EventBus) (context.Context, error) {
	return reqCtx, nil
}

func (s *GenericStep) Rollback(reqCtx context.Context, bus eventbus.EventBus) (context.Context, error) {
	return reqCtx, nil
}

func (s *GenericStep) GetInfo() StepInfo {
	return s.StepInfo
}

func (s *GenericStep) GetSteps() []Step {
	return s.Steps
}

func (s *GenericStep) SetState(state State) {
	s.State = state
}

func (s *GenericStep) SetSuccessFn(fn func(context.Context, Step)) {
	s.successFn = fn
}

func (s *GenericStep) SetFailFn(fn func(context.Context, Step, error)) {
	s.failFn = fn
}

func (s *GenericStep) SetContextHandler(fn func([]context.Context) context.Context) {
	s.contextHandler = fn
}

func (s *GenericStep) SetErrorHandler(fn func([]error) error) {
	s.errorHandler = fn
}

func (s *GenericStep) SetEventHandler(fn func(eventbus.Event) error) {
	s.eventHandler = eventbus.Handler(fn)
}

func (s *GenericStep) SetLogger(log rabbit.Logger) {
	s.Log = log
}

func (s *GenericStep) Success(reqCtx context.Context) {
	if s.successFn == nil {
		s.SetState(StateCompleted)
	} else {
		s.successFn(reqCtx, s)
	}
}

func (s *GenericStep) Fail(reqCtx context.Context, err error) {
	if s.failFn == nil {
		s.SetState(StateFailed)
	} else {
		s.failFn(reqCtx, s, err)
	}
}

///////////////////////////////////////////////////////////////////////
//Utility functions for Step
type StepPredicate func(Step) bool

func Filter(s Step, pred StepPredicate) []Step {
	var res []Step
	for _, step := range s.GetSteps() {
		if pred(step) {
			res = append(res, step)
		}
	}
	return res
}

func GetActiveSteps(step Step, deepest bool) []Step {
	return FindSteps(step, func(s Step) bool {
		return s.GetInfo().State == StateProcessing || s.GetInfo().State == StateRollingback
	}, deepest)
}

func FindSteps(step Step, pred StepPredicate, deepest bool) []Step {
	if !pred(step) {
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
			if pred(thisStep) {
				if deepest {
					return FindSteps(thisStep, pred, true)
				}
				//look for the first level active steps
				res = append(res, thisStep)
			}
		}

		if deepest {
			res = append(res, step)
		}
		return res

	default:
		res = append(res, s)
		return res
	}

}
