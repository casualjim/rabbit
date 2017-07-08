///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/casualjim/rabbit"
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

	bus := eventbus.New(rabbit.GoLog(os.Stderr, "", 0))

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
	bus := eventbus.New(rabbit.GoLog(os.Stderr, "", 0))

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

//set up a step with a few substeps, the deepest active step is S112
func setupParallelStepSuccess() *ParallelStep {
	return NewParallelStep(
		StepInfo{Name: "Create Clusters", State: StateWaiting},
		nil,
		testAggContext,
		testErrorHandler,
		testEventHandlerFn,
		NewSeqStep(
			StepInfo{Name: "Create Cluster 1", State: StateWaiting},
			nil,
			testAggContext,
			testErrorHandler,
			nil,
			NewParallelStep(
				StepInfo{Name: "Create Leader", State: StateWaiting},
				nil,
				testAggContext,
				testErrorHandler,
				nil,
				newTestUnitStep(StepInfo{Name: "Create Leader 1", State: StateWaiting}, 3, "", Leader, false, "10.0.0.1"),
				newTestUnitStep(StepInfo{Name: "Create Leader 2", State: StateWaiting}, 4, "", Leader, false, "10.0.0.2"),
			),
			newTestUnitStep(StepInfo{Name: "Create Worker 1", State: StateWaiting}, 1, "", Worker, false, "10.0.0.3"),
		),
	)
}

//set up a step with a few substeps, the deepest active step is S112
func setupParallelStepFailFast() *ParallelStep {
	return NewParallelStep(
		StepInfo{Name: "Create Cluster", State: StateWaiting},
		nil,

		testAggContext,
		testErrorHandler,
		testEventHandlerFn,
		newTestUnitStep(StepInfo{Name: "Create Leader 1", State: StateWaiting}, 1, "", Leader, true, "10.0.0.1"),
	)
}

//set up a step with a few substeps, the deepest active step is S112
func setupParallelStepFail() *ParallelStep {
	return NewParallelStep(
		StepInfo{Name: "Create Clusters", State: StateWaiting},
		nil,
		testAggContext,
		testErrorHandler,
		testEventHandlerFn,
		NewSeqStep(
			StepInfo{Name: "Create Cluster 1", State: StateWaiting},
			nil,
			testAggContext,
			testErrorHandler,
			testEventHandlerFn,
			NewParallelStep(
				StepInfo{Name: "Create Leader", State: StateWaiting},
				nil,

				testAggContext,
				testErrorHandler,
				testEventHandlerFn,
				newTestUnitStep(StepInfo{Name: "Create Leader 1", State: StateWaiting}, 3, "", Leader, true, "10.0.0.1"),
				newTestUnitStep(StepInfo{Name: "Create Leader 2", State: StateWaiting}, 4, "", Leader, true, "10.0.0.2"),
			),
			newTestUnitStep(StepInfo{Name: "Create Worker 1", State: StateWaiting}, 1, "", Worker, false, "10.0.0.3"),
		),
	)
}

func TestParallelStepRunSuccess(t *testing.T) {
	reqCtx := context.Background()

	parallelStep := setupParallelStepSuccess()
	bus := eventbus.New(rabbit.GoLog(os.Stderr, "", 0))

	reqCtx, err := parallelStep.Run(reqCtx, bus)

	//check the value is passed through context
	runtimeresult, _ := testFromContext(reqCtx)
	assert.Equal(t, runtimeresult.LeaderIP[0], "10.0.0.1")
	assert.Equal(t, runtimeresult.LeaderIP[1], "10.0.0.2")
	assert.Equal(t, runtimeresult.WorkerIP[0], "10.0.0.3")

	assert.Equal(t, err, nil)
}

func TestParallelStepRunFailFast(t *testing.T) {
	reqCtx := context.Background()

	//setup a paral step which fails fast

	parallelStep := setupParallelStepFailFast()

	var wg sync.WaitGroup
	wg.Add(1)

	var err error
	bus := eventbus.New(rabbit.GoLog(os.Stderr, "", 0))

	go func() {
		_, err = parallelStep.Run(reqCtx, bus)
		wg.Done()
	}()

	wg.Wait()

	//verify error
	assert.Error(t, err)
	assert.EqualError(t, err, failedMsg("Create Leader 1"))
}

func TestParallelStepRunFail(t *testing.T) {
	reqCtx := context.Background()
	expErrStr := []string{
		failedMsg("Create Leader 1"),
		failedMsg("Create Leader 2"),
	}

	parallelStep := setupParallelStepFail()
	//does not work because S111 failed but S112 succeeded, so ctx and err channels both got values

	var wg sync.WaitGroup
	wg.Add(1)

	var err error
	bus := eventbus.New(rabbit.GoLog(os.Stderr, "", 0))

	go func() {
		_, err = parallelStep.Run(reqCtx, bus)
		wg.Done()
	}()

	wg.Wait()

	//verify error
	assert.Error(t, err)
	for i := 0; i < len(expErrStr); i++ {
		assert.Contains(t, err.Error(), expErrStr[i])
	}
}

func TestParallelStepRunCancel(t *testing.T) {

	expError := []string{
		canceledMsg("Create Clusters"),
		canceledMsg("Create Cluster 1"),
		canceledMsg("Create Leader 1"),
		canceledMsg("Create Leader 2"),
		canceledMsg("Create Leader"),
	}

	reqCtx := context.Background()
	reqCtx, cancel := context.WithCancel(reqCtx)
	defer cancel()

	parallelStep := setupParallelStepSuccess()

	var wg sync.WaitGroup
	wg.Add(1)

	var err error
	bus := eventbus.New(rabbit.GoLog(os.Stderr, "", 0))

	go func() {
		reqCtx, err = parallelStep.Run(reqCtx, bus)
		wg.Done()
	}()

	//send cancel signal
	go func() {
		time.Sleep(time.Second * 2)
		cancel()
	}()

	wg.Wait()
	//check the value is passed through context
	for i := 0; i < len(expError); i++ {
		assert.Contains(t, err.Error(), expError[i])
	}
	//we only set the root step to processing the cancel event, so only expect to see 2 here
	//one for "Create Leader 1", one for "Create Leader 2"
	assert.EqualValues(t, atomic.LoadInt64(&seenCancel), 2)
}
