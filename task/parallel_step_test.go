///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/casualjim/rabbit/eventbus"
	"github.com/stretchr/testify/assert"
)

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
	bus := eventbus.New(logrus.New())

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
	bus := eventbus.New(logrus.New())

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
	bus := eventbus.New(logrus.New())

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
		canceledMsg("Create Cluster 1"),
		canceledMsg("Create Leader 1"),
		canceledMsg("Create Leader 2"),
	}

	reqCtx := context.Background()
	reqCtx, cancel := context.WithCancel(reqCtx)
	defer cancel()

	parallelStep := setupParallelStepSuccess()

	var wg sync.WaitGroup
	wg.Add(1)

	var err error
	bus := eventbus.New(logrus.New())

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
