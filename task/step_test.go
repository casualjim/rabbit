///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"testing"

	"github.com/casualjim/rabbit/joint"
	"github.com/stretchr/testify/assert"
)

func newTestError(msg string) *joint.Error {
	return &joint.Error{
		Message: msg,
	}
}

type successStep struct {
	GenericStep
}

func newSuccessStep(stepInfo StepInfo) *successStep {
	return &successStep{
		GenericStep{StepInfo: stepInfo},
	}
}

func (s *successStep) Run(reqCtx context.Context) (context.Context, *joint.Error) {
	s.State = StateCompleted
	runTime, _ := FromContext(reqCtx)
	runTime.Log = append(runTime.Log, s.Name+" succeed")
	reqCtx = NewContext(reqCtx, runTime)
	return reqCtx, nil
}

type failedStep struct {
	GenericStep
}

func newFailedStep(stepInfo StepInfo) *failedStep {
	return &failedStep{
		GenericStep{StepInfo: stepInfo},
	}
}

func (s *failedStep) Run(reqCtx context.Context) (context.Context, *joint.Error) {
	s.State = StateFailed
	runTime, _ := FromContext(reqCtx)
	runTime.Cause = append(runTime.Cause, s.Name)
	runTime.Log = append(runTime.Log, "failed step")
	reqCtx = NewContext(reqCtx, runTime)
	return reqCtx, newTestError("failed step")
}

func setupSeqStepFail() *SeqStep {
	stepOpts := NewStepInfo("TestStep")

	return NewSeqStep(
		stepOpts,
		newSuccessStep(NewStepInfo("Step1")),
		newFailedStep(NewStepInfo("Step2")),
	)
}

func setupSeqStepSucceess() *SeqStep {
	stepOpts := NewStepInfo("TestStep")

	return NewSeqStep(
		stepOpts,
		newSuccessStep(NewStepInfo("Step1")),
		newSuccessStep(NewStepInfo("Step2")),
	)
}

//set up a step with a few substeps, the deepest active step is S112
func setupSeqStep() *SeqStep {
	return NewSeqStep(
		StepInfo{Name: "S", State: StateProcessing},
		NewSeqStep(
			StepInfo{Name: "S1", State: StateProcessing},
			NewSeqStep(
				StepInfo{Name: "S11", State: StateProcessing},
				NewSeqStep(StepInfo{Name: "S111", State: StateCompleted}),
				NewSeqStep(StepInfo{Name: "S112", State: StateProcessing}),
			),
			NewSeqStep(
				StepInfo{Name: "S12", State: StateNone},
			),
		),
		NewSeqStep(
			StepInfo{Name: "S2", State: StateNone},
		),
		NewSeqStep(
			StepInfo{Name: "S3", State: StateNone},
			NewSeqStep(
				StepInfo{Name: "S31", State: StateNone},
				NewSeqStep(StepInfo{Name: "S311", State: StateNone}),
				NewSeqStep(StepInfo{Name: "S312", State: StateNone}),
			),
			NewSeqStep(
				StepInfo{Name: "S32", State: StateNone},
			),
		),
	)
}

func TestSeqStepRunFail(t *testing.T) {
	runtime := TaskRunTime{
		ClusterID: "1",
	}

	reqCtx := context.Background()
	reqCtx = NewContext(reqCtx, runtime)

	seqStep := setupSeqStepFail()
	reqCtx, err := seqStep.Run(reqCtx)

	//check the value is passed through context
	runtimeresult, _ := FromContext(reqCtx)
	assert.Equal(t, runtimeresult.Log[0], "Step1 succeed")
	assert.Equal(t, runtimeresult.Log[1], "failed step")
	assert.Equal(t, runtimeresult.Cause[0], "Step2")
	assert.Equal(t, err, newTestError("failed step"))
	assert.Equal(t, seqStep.State, StateFailed)
}

func TestSeqStepRunSuccess(t *testing.T) {
	runtime := TaskRunTime{
		ClusterID: "1",
	}

	reqCtx := context.Background()
	reqCtx = NewContext(reqCtx, runtime)

	seqStep := setupSeqStepSucceess()
	reqCtx, _ = seqStep.Run(reqCtx)

	//check the value is passed through context
	runtimeresult, _ := FromContext(reqCtx)
	assert.Equal(t, runtimeresult.Log[0], "Step1 succeed")
	assert.Equal(t, runtimeresult.Log[1], "Step2 succeed")
	assert.Equal(t, seqStep.State, StateCompleted)
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
