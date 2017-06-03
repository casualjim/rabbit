///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"testing"
	"time"

	"github.com/casualjim/rabbit/joint"
	"github.com/stretchr/testify/assert"
)

const (
	TaskTypeTest TaskType = "test task"
)

func Success(*Task, Step) {

}
func Fail(*Task, Step, *joint.Error) {

}

type dummyStep struct {
	GenericStep
}

func (s *dummyStep) Run(reqCtx context.Context) (context.Context, *joint.Error) {
	s.State = StateProcessing
	go func() {
		time.Sleep(time.Second * 2)
		s.State = StateCompleted
	}()
	runTime, _ := FromContext(reqCtx)
	runTime.Log = append(runTime.Log, "finished dummy step")
	reqCtx = NewContext(reqCtx, runTime)
	return reqCtx, nil
}

func newDummyStep() *dummyStep {
	return &dummyStep{*NewGenericStep(NewStepInfo("dummy step"), nil)}
}

func TestTaskRun(t *testing.T) {
	runtime := TaskRunTime{
		ClusterID: "1",
	}
	taskOpts := TaskOpts{RunTime: runtime, Type: TaskTypeTest, SuccessFn: Success, FailFn: Fail, Ctx: context.Background()}

	task, _ := NewTask(taskOpts, newDummyStep())

	state := task.CheckStatus()
	//job is just created, not running
	assert.Equal(t, state, StateCreated)
	assert.Equal(t, task.Type, TaskTypeTest)

	//job running
	go task.Run()
	time.Sleep(time.Second * 1)
	state = task.CheckStatus()
	assert.Equal(t, state, StateProcessing)

	//waiting for steps to finish and check status
	time.Sleep(time.Second * 2)
	state = task.CheckStatus()
	assert.Equal(t, state, StateCompleted)

	//check the value is passed through context
	runtimeresult, _ := FromContext(task.ctx)
	assert.Equal(t, runtimeresult.Log[0], "finished dummy step")
}
