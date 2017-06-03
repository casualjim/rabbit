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

func newSuccessStep(stepOpts StepOpts) *successStep {
	return &successStep{
		GenericStep{stepOpts, nil},
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

func newFailedStep(stepOpts StepOpts) *failedStep {
	return &failedStep{
		GenericStep{stepOpts, nil},
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
	stepOpts := StepOpts{
		Name:  "TestStep",
		State: StateNone,
	}

	return NewSeqStep(
		stepOpts,
		newSuccessStep(StepOpts{Name: "Step1", State: StateNone}),
		newFailedStep(StepOpts{Name: "Step2", State: StateNone}),
	)
}

func setupSeqStepSucceess() *SeqStep {
	stepOpts := StepOpts{
		Name:  "TestStep",
		State: StateNone,
	}

	return NewSeqStep(
		stepOpts,
		newSuccessStep(StepOpts{Name: "Step1", State: StateNone}),
		newSuccessStep(StepOpts{Name: "Step2", State: StateNone}),
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
