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

// Conditional step
// step that executes the condFn, that returns
// a negative integer, zero, or a positive integer
// if the result is negative, the lStep Step is run
// if the result is positive, the rStep Step is run
// else no step is run

type ConditionalStep struct {
	condFn func(ctx context.Context) (context.Context, int, error)
	lStep  Step
	rStep  Step
	log    rabbit.Logger
}

func NewConditionalStep(stepInfo StepInfo,
	log rabbit.Logger,
	lStep, rStep Step,
	condFn func(context.Context) (context.Context, int, error)) *ConditionalStep {

	c := &ConditionalStep{
		lStep:  lStep,
		rStep:  rStep,
		condFn: condFn,
		log:    log,
	}
	return c
}

func (s *ConditionalStep) Run(reqCtx context.Context, bus eventbus.EventBus) (context.Context, error) {
	reqCtx, i, err := s.condFn(reqCtx)
	if err != nil {
		s.log.Printf("step failed, %s", err.Error())
		return reqCtx, err
	}
	if i < 0 {
		s.log.Printf("running lStep  %s", s.lStep.GetInfo().Name)
		return s.lStep.Run(reqCtx, bus)
	} else if i > 0 {
		s.log.Printf("running rStep  %s", s.rStep.GetInfo().Name)
		return s.rStep.Run(reqCtx, bus)
	}
	s.log.Printf("skipping lStep and rStep")
	return reqCtx, nil
}
