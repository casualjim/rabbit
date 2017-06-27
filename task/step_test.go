///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"errors"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/casualjim/rabbit/eventbus"
	"github.com/stretchr/testify/assert"
)

//testAggContext appends c2's Runtime to c1
func testAggContext(ctxs []context.Context) context.Context {
	//find the first context in the list which has runtime
	var returnCtx context.Context
	var runtime *testClusterRunTime

	for _, ctx := range ctxs {
		if ctx != nil {
			runtime1, ok1 := testFromContext(ctx)
			if runtime == nil {
				if ok1 {
					runtime = &runtime1
				}
			} else {
				runtime.LeaderIP = append(runtime.LeaderIP, runtime1.LeaderIP...)
				runtime.WorkerIP = append(runtime.WorkerIP, runtime1.WorkerIP...)
			}
			returnCtx = ctx
		}
	}

	if runtime == nil {
		return returnCtx
	}

	return testNewContext(returnCtx, *runtime)
}

//just concatinate the errors
func testErrorHandler(errs []error) error {
	var err error
	if errs == nil {
		return nil
	}
	for _, e := range errs {
		if e == nil {
			continue
		}
		if err == nil {
			err = e
		} else {
			err = errors.New(err.Error() + "\n" + e.Error())
		}
	}
	return err
}

func setupSeqStepFail() *SeqStep {
	stepOpts := NewStepInfo("TestStep")

	return NewSeqStep(
		stepOpts, nil, testAggContext, testErrorHandler, nil,
		newTestUnitStep(StepInfo{Name: "create leader 1", State: StateWaiting}, 2, "", Leader, true, "10.0.0.1"),
		newTestUnitStep(StepInfo{Name: "create leader 2", State: StateWaiting}, 2, "", Leader, false, "10.0.0.2"),
	)
}

func setupSeqStepSucceess() *SeqStep {
	stepOpts := NewStepInfo("TestStep")

	return NewSeqStep(
		stepOpts, nil, nil, nil, nil,
		newTestUnitStep(StepInfo{Name: "create leader 1", State: StateWaiting}, 2, "", Leader, false, "10.0.0.1"),
		newTestUnitStep(StepInfo{Name: "create leader 2", State: StateWaiting}, 2, "", Leader, false, "10.0.0.2"),
	)
}

//set up a step with a few substeps, the deepest active step is S112
func setupSeqStep() *SeqStep {
	return NewSeqStep(
		StepInfo{Name: "S", State: StateProcessing}, nil, nil, nil, nil,
		NewSeqStep(
			StepInfo{Name: "S1", State: StateProcessing}, nil, nil, nil, nil,
			NewSeqStep(
				StepInfo{Name: "S11", State: StateProcessing}, nil, nil, nil, nil,
				NewSeqStep(StepInfo{Name: "S111", State: StateCompleted}, nil, nil, nil, nil),
				NewSeqStep(StepInfo{Name: "S112", State: StateProcessing}, nil, nil, nil, nil),
			),
			NewSeqStep(
				StepInfo{Name: "S12", State: StateWaiting}, nil, nil, nil, nil,
			),
		),
		NewSeqStep(
			StepInfo{Name: "S2", State: StateWaiting}, nil, nil, nil, nil,
		),
		NewSeqStep(
			StepInfo{Name: "S3", State: StateWaiting}, nil, nil, nil, nil,
			NewSeqStep(
				StepInfo{Name: "S31", State: StateWaiting}, nil, nil, nil, nil,
				NewSeqStep(StepInfo{Name: "S311", State: StateWaiting}, nil, nil, nil, nil),
				NewSeqStep(StepInfo{Name: "S312", State: StateWaiting}, nil, nil, nil, nil),
			),
			NewSeqStep(
				StepInfo{Name: "S32", State: StateWaiting}, nil, nil, nil, nil,
			),
		),
	)
}

func TestSeqStepRunFail(t *testing.T) {
	runtime := testClusterRunTime{}

	reqCtx := context.Background()
	reqCtx = testNewContext(reqCtx, runtime)

	seqStep := setupSeqStepFail()

	bus := eventbus.New(logrus.New())

	reqCtx, err := seqStep.Run(reqCtx, bus)

	//check the value is passed through context
	assert.Equal(t, seqStep.GetState(), StateFailed)
	assert.Error(t, err)
	assert.EqualError(t, err, failedMsg("create leader 1"))
}

func TestSeqStepRunSuccess(t *testing.T) {
	runtime := testClusterRunTime{}

	reqCtx := context.Background()
	reqCtx = testNewContext(reqCtx, runtime)

	seqStep := setupSeqStepSucceess()
	bus := eventbus.New(logrus.New())

	reqCtx, err := seqStep.Run(reqCtx, bus)

	//check the value is passed through context
	runtimeresult, _ := testFromContext(reqCtx)
	assert.Equal(t, runtimeresult.LeaderIP[0], "10.0.0.1")
	assert.Equal(t, runtimeresult.LeaderIP[1], "10.0.0.2")

	assert.Equal(t, seqStep.GetState(), StateCompleted)
	stepsCompleted := FindSteps(seqStep, func(s Step) bool {
		if s.GetInfo().State == StateCompleted {
			return true
		}
		return false
	}, false)
	assert.Equal(t, len(stepsCompleted), len(seqStep.Steps))
	assert.NoError(t, err)
}

func TestGetActiveSteps(t *testing.T) {

	seqStep := setupSeqStep()
	activeSteps := GetActiveSteps(seqStep, true)
	for _, step := range activeSteps {
		assert.Equal(t, step.GetInfo().Name, "S112")
	}

	activeSteps = GetActiveSteps(seqStep, false)
	for _, step := range activeSteps {
		assert.Equal(t, step.GetInfo().Name, "S1")
	}
}
