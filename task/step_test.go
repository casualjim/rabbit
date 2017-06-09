///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type successStep struct {
	GenericStep
}

func newSuccessStep(stepInfo StepInfo) *successStep {
	return &successStep{
		GenericStep{StepInfo: stepInfo},
	}
}

func (s *successStep) Run(reqCtx context.Context) (context.Context, error) {
	s.State = StateCompleted
	runTime, _ := testFromContext(reqCtx)
	runTime.Log = append(runTime.Log, s.Name+" succeed")
	reqCtx = testNewContext(reqCtx, runTime)
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

func (s *failedStep) Run(reqCtx context.Context) (context.Context, error) {
	s.State = StateFailed
	runTime, _ := testFromContext(reqCtx)
	runTime.Cause = append(runTime.Cause, s.Name)
	runTime.Log = append(runTime.Log, "failed step")
	reqCtx = testNewContext(reqCtx, runTime)
	return reqCtx, errors.New("failed step")
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
	runtime := testRunTime{}

	reqCtx := context.Background()
	reqCtx = testNewContext(reqCtx, runtime)

	seqStep := setupSeqStepFail()
	reqCtx, err := seqStep.Run(reqCtx)

	//check the value is passed through context
	runtimeresult, _ := testFromContext(reqCtx)
	assert.Equal(t, runtimeresult.Log[0], "Step1 succeed")
	assert.Equal(t, runtimeresult.Log[1], "failed step")
	assert.Equal(t, runtimeresult.Cause[0], "Step2")
	assert.Equal(t, seqStep.State, StateFailed)
	assert.Error(t, err)
	assert.Equal(t, err, errors.New("failed step"))
}

func TestSeqStepRunSuccess(t *testing.T) {
	runtime := testRunTime{}

	reqCtx := context.Background()
	reqCtx = testNewContext(reqCtx, runtime)

	seqStep := setupSeqStepSucceess()
	reqCtx, err := seqStep.Run(reqCtx)

	//check the value is passed through context
	runtimeresult, _ := testFromContext(reqCtx)
	assert.Equal(t, runtimeresult.Log[0], "Step1 succeed")
	assert.Equal(t, runtimeresult.Log[1], "Step2 succeed")
	assert.Equal(t, seqStep.State, StateCompleted)
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

///////////////////////
type testUnitStep struct {
	GenericStep
	val  string
	wait int
	err  string
	fail bool
}

func newTestUnitStep(stepInfo StepInfo, v string, wait int, err string, fail bool) *testUnitStep {
	return &testUnitStep{
		GenericStep: *NewGenericStep(stepInfo, nil),
		val:         v,
		wait:        wait,
		err:         err,
		fail:        fail,
	}
}

func (s *testUnitStep) Run(ctx context.Context) (context.Context, error) {
	s.State = StateProcessing
	runtime := testRunTime{}
	if s.err != "" {
		runtime = testRunTime{Cause: []string{"step " + s.val + " failed\n"}}
	}
	runtime = testRunTime{Log: []string{"step " + s.val + " completed\n"}}
	var wg sync.WaitGroup
	wg.Add(1)

	ctxc := make(chan context.Context, 1)
	go func() {
		for i := 1; i <= s.wait; i++ {
			time.Sleep(time.Second)
		}
		ctx = testNewContext(ctx, runtime)
		ctxc <- ctx
		wg.Done()
	}()

	var canceledErr error
	go func() {
		select {
		case <-ctx.Done():
			canceledErr = errors.New("step " + s.Name + " canceled")
			return
		}
	}()
	wg.Wait()

	select {
	case newCtx := <-ctxc:
		ctx = newCtx
	}
	if canceledErr == nil {
		if s.fail {
			s.State = StateFailed
			return ctx, errors.New("step " + s.Name + " failed")
		}
		s.State = StateCompleted

		return ctx, nil
	} else {
		s.State = StateCanceled
		return ctx, canceledErr
	}

}

//testAggContext appends c2's Runtime to c1
func testAggContext(c1 context.Context, c2 context.Context) context.Context {
	runtime1, ok1 := testFromContext(c1)
	runtime2, ok2 := testFromContext(c2)
	if !ok1 && !ok2 {
		return c1
	}

	if ok1 && ok2 {
		runtime := testRunTime{}
		runtime.Log = append(runtime1.Log, runtime2.Log...)
		runtime.Cause = append(runtime1.Cause, runtime2.Cause...)
		return testNewContext(c1, runtime)
	}

	if ok1 {
		return c1
	}

	return testNewContext(c1, runtime2)
}

//just concatinate the errors
func testAggError(errs []error) error {
	var err error
	if errs == nil {
		return nil
	}
	for _, e := range errs {
		if err == nil {
			err = e
		} else {
			err = errors.New(err.Error() + "\n" + e.Error())
		}
	}
	return err
}

//set up a step with a few substeps, the deepest active step is S112
func setupParalStepSuccess() *ParalStep {
	return NewParalStep(
		StepInfo{Name: "S", State: StateNone},
		testAggContext,
		testAggError,
		NewParalStep(
			StepInfo{Name: "S1", State: StateNone},
			testAggContext,
			testAggError,
			NewParalStep(
				StepInfo{Name: "S11", State: StateNone},
				testAggContext,
				testAggError,
				newTestUnitStep(StepInfo{Name: "S111", State: StateNone}, "S111", 3, "", false),
				newTestUnitStep(StepInfo{Name: "S112", State: StateNone}, "S112", 4, "", false),
			),
			newTestUnitStep(StepInfo{Name: "S12", State: StateNone}, "S12", 1, "", false),
		),
	)
}

//set up a step with a few substeps, the deepest active step is S112
func setupParalStepFailFast() *ParalStep {
	return NewParalStep(
		StepInfo{Name: "S", State: StateNone},
		testAggContext,
		testAggError,
		newTestUnitStep(StepInfo{Name: "S11", State: StateNone}, "S11", 1, "", true),
	)
}

//set up a step with a few substeps, the deepest active step is S112
func setupParalStepFail() *ParalStep {
	return NewParalStep(
		StepInfo{Name: "S", State: StateNone},
		testAggContext,
		testAggError,
		NewParalStep(
			StepInfo{Name: "S1", State: StateNone},
			testAggContext,
			testAggError,
			NewParalStep(
				StepInfo{Name: "S11", State: StateNone},
				testAggContext,
				testAggError,
				newTestUnitStep(StepInfo{Name: "S111", State: StateNone}, "S111", 3, "", true),
				newTestUnitStep(StepInfo{Name: "S112", State: StateNone}, "S112", 4, "", false),
			),
			newTestUnitStep(StepInfo{Name: "S12", State: StateNone}, "S12", 1, "", true),
		),
	)
}

func TestParalStepRunSuccess(t *testing.T) {

	resultExp := []string{
		"step S12 completed\n",
		"step S111 completed\n",
		"step S112 completed\n",
	}

	reqCtx := context.Background()

	paralStep := setupParalStepSuccess()

	reqCtx, err := paralStep.Run(reqCtx)

	//check the value is passed through context
	runtimeresult, _ := testFromContext(reqCtx)
	assert.Equal(t, runtimeresult.Log, resultExp)
	assert.Equal(t, err, nil)
}

func TestParalStepRunCancel(t *testing.T) {

	expError := []string{
		"step S1 canceled",
		"step S11 canceled",
		//"step S32 canceled", //S32 has finished before cancel() starts
		"step S111 canceled",
		"step S112 canceled",
	}

	reqCtx := context.Background()
	reqCtx, cancel := context.WithCancel(reqCtx)
	defer cancel()

	paralStep := setupParalStepSuccess()

	var wg sync.WaitGroup
	wg.Add(1)

	var err error

	go func() {
		reqCtx, err = paralStep.Run(reqCtx)
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
}

func TestParalStepRunFailFast(t *testing.T) {
	reqCtx := context.Background()

	//setup a paral step which fails fast

	paralStep := setupParalStepFailFast()

	var wg sync.WaitGroup
	wg.Add(1)

	var err error

	go func() {
		_, err = paralStep.Run(reqCtx)
		wg.Done()
	}()

	wg.Wait()

	//verify error
	assert.Error(t, err)
	assert.EqualError(t, err, "step S11 failed")
}

func TestParalStepRunFail(t *testing.T) {
	reqCtx := context.Background()
	expErrStr := []string{
		"step S111 failed",
		"step S12 failed",
	}

	paralStep := setupParalStepFail()
	//does not work because S111 failed but S112 succeeded, so ctx and err channels both got values

	var wg sync.WaitGroup
	wg.Add(1)

	var err error

	go func() {
		_, err = paralStep.Run(reqCtx)
		wg.Done()
	}()

	wg.Wait()

	//verify error
	assert.Error(t, err)
	for i := 0; i < len(expErrStr); i++ {
		assert.Contains(t, err.Error(), expErrStr[i])
	}
}
