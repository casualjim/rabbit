///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	TaskTypeTest TaskType = "test task"
)

// testRunTime metadata to describe the Task
type testRunTime struct {
	// cause is for the aggregated errors
	Cause []string `json:"cause,omitempty"`
	// log is for the aggregated logs
	Log []string `json:"log,omitempty"`
}

type testKey int

const runtimeKey testKey = 0

func testNewContext(ctx context.Context, runtime testRunTime) context.Context {
	return context.WithValue(ctx, runtimeKey, runtime)
}

func testFromContext(ctx context.Context) (testRunTime, bool) {
	// ctx.Value returns nil if ctx has no value for the key;
	value := ctx.Value(runtimeKey)
	if value != nil {
		return value.(testRunTime), true
	}
	return testRunTime{}, false
}

func Success(*Task) {

}
func Fail(*Task, error) {

}

type dummyStep struct {
	GenericStep
	fail bool
}

func (s *dummyStep) Run(reqCtx context.Context) (context.Context, error) {
	s.State = StateProcessing

	time.Sleep(time.Second * 2)
	s.State = StateCompleted
	runTime, _ := testFromContext(reqCtx)
	runTime.Log = append(runTime.Log, "finished dummy step")

	reqCtx = testNewContext(reqCtx, runTime)
	if s.fail {
		s.State = StateFailed
		return reqCtx, errors.New("dummy step failed")
	}
	return reqCtx, nil
}

func newDummyStep(fail bool) *dummyStep {
	return &dummyStep{GenericStep: *NewGenericStep(NewStepInfo("dummy step"), nil), fail: fail}
}

func TestTaskRunSucceed(t *testing.T) {

	taskOpts := TaskOpts{Type: TaskTypeTest, SuccessFn: Success, FailFn: Fail, Ctx: context.Background()}

	task, _ := NewTask(taskOpts, newDummyStep(false))

	state := task.CheckStatus()
	//job is just created, not running
	assert.Equal(t, state, StateCreated)
	assert.Equal(t, task.Type, TaskTypeTest)

	//job running
	result := make(chan error)
	go func() {
		err := task.Run()
		result <- err
		close(result)
	}()

	time.Sleep(time.Second * 1)
	state = task.CheckStatus()
	assert.Equal(t, state, StateProcessing)

	//time.Sleep(time.Second * 1)
	err := <-result
	//waiting for steps to finish and check status
	state = task.CheckStatus()
	assert.Equal(t, state, StateCompleted)
	assert.NoError(t, err)
	//check the value is passed through context
	runtimeresult, _ := testFromContext(task.ctx)
	assert.Equal(t, runtimeresult.Log[0], "finished dummy step")
}

func TestTaskRunFail(t *testing.T) {

	taskOpts := TaskOpts{Type: TaskTypeTest, SuccessFn: Success, FailFn: Fail, Ctx: context.Background()}

	task, _ := NewTask(taskOpts, newDummyStep(true))

	state := task.CheckStatus()
	//job is just created, not running
	assert.Equal(t, state, StateCreated)
	assert.Equal(t, task.Type, TaskTypeTest)

	//job running
	result := make(chan error)
	go func() {
		err := task.Run()
		result <- err
		close(result)
	}()

	time.Sleep(time.Second * 1)
	state = task.CheckStatus()
	assert.Equal(t, state, StateProcessing)

	//time.Sleep(time.Second * 1)
	err := <-result
	//waiting for steps to finish and check status
	state = task.CheckStatus()
	assert.Equal(t, state, StateFailed)
	assert.EqualError(t, err, "dummy step failed")
}

func TestTaskRunCancel(t *testing.T) {

	taskOpts := TaskOpts{Type: TaskTypeTest, SuccessFn: Success, FailFn: Fail, Ctx: context.Background()}

	task, _ := NewTask(taskOpts, newTestUnitStep(NewStepInfo("TestStep"), "TestStep", 2, "", false))
	ctx, cancel := context.WithCancel(task.ctx)
	task.ctx = ctx

	state := task.CheckStatus()
	//job is just created, not running
	assert.Equal(t, state, StateCreated)
	assert.Equal(t, task.Type, TaskTypeTest)

	//job running
	result := make(chan error)
	go func() {
		err := task.Run()
		result <- err
		close(result)
	}()

	time.Sleep(time.Second * 1)
	cancel()
	state = task.CheckStatus()
	fmt.Printf("------- %s\n", task.TaskStep.GetInfo().State)
	assert.Equal(t, state, StateProcessing)

	//time.Sleep(time.Second * 1)
	err := <-result
	//waiting for steps to finish and check status
	state = task.CheckStatus()
	assert.Equal(t, state, StateCanceled)
	assert.EqualError(t, err, "step TestStep canceled")
}
