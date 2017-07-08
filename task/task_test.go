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

const (
	TaskTypeTest TaskType = "test task"
)

/////////runtime///////////
type testClusterRunTime struct {
	ClusterID string
	LeaderIP  []string
	WorkerIP  []string
}

type testKey int

const runtimeKey testKey = 0

func testNewContext(ctx context.Context, runtime testClusterRunTime) context.Context {
	return context.WithValue(ctx, runtimeKey, runtime)
}

func testFromContext(ctx context.Context) (testClusterRunTime, bool) {
	// ctx.Value returns nil if ctx has no value for the key;
	value := ctx.Value(runtimeKey)
	if value != nil {
		return value.(testClusterRunTime), true
	}
	return testClusterRunTime{}, false
}

/////////event//////
var seenCancel int64

// var seenCancelL = new(sync.Mutex)

func testEventHandlerFn(evt eventbus.Event) error {
	// seenCancelL := new(sync.Mutex)
	// seenCancelL.Lock()
	//evts[0] = evt
	switch evt.Args.(type) {
	case clusterEvent:
		atomic.AddInt64(&seenCancel, 1)
	}
	// seenCancelL.Unlock()
	return nil
}

type clusterEvent struct {
	Name     string
	TaskID   TaskID
	TaskType TaskType
	StepName string
	Status   string
}

func newClusterEvent(name string, args clusterEvent) eventbus.Event {
	return eventbus.Event{
		Name: name,
		Args: args,
	}
}

///////////step////////////

func failedMsg(name string) string {
	return "step " + name + " failed"
}

func canceledMsg(name string) string {
	return "step " + name + " canceled"
}

func successMsg(name string) string {
	return "step " + name + "completed"
}

type stepRole uint8

const (
	Leader stepRole = iota
	Worker
)

type testUnitStep struct {
	GenericStep
	wait int
	err  string
	fail bool
	role stepRole
	ip   string
}

func newTestUnitStep(stepInfo StepInfo, wait int, err string, role stepRole, fail bool, ip string) *testUnitStep {
	return &testUnitStep{
		GenericStep: *NewGenericStep(stepInfo, nil),
		wait:        wait,
		err:         err,
		role:        role,
		fail:        fail,
		ip:          ip,
	}
}

func (s *testUnitStep) Run(ctx context.Context, bus eventbus.EventBus) (context.Context, error) {
	s.SetState(StateProcessing)
	runtime, _ := testFromContext(ctx)
	timeout := time.Second * time.Duration(s.wait)
	log := rabbit.GoLog(os.Stderr, "", 0)

	var wg1 sync.WaitGroup
	ctxc := make(chan context.Context, 1)
	wg1.Add(1)

	go func() {
		select {
		case <-time.After(timeout):
			if !s.fail {
				if s.role == Leader {
					runtime.LeaderIP = append(runtime.LeaderIP, s.ip)
				} else {
					runtime.WorkerIP = append(runtime.WorkerIP, s.ip)
				}
			}
			ctxc <- testNewContext(ctx, runtime)
			log.Infof("step %s execute..finished waiting.\n", s.GetName())
			wg1.Done()
			return
		}
	}()

	var wg2 sync.WaitGroup
	var canceledErr error
	wg2.Add(1)
	go func() {
		select {
		case <-ctx.Done():
			canceledErr = errors.New(canceledMsg(s.GetName()))
			bus.Publish(
				newClusterEvent(
					"ClusterEvent",
					clusterEvent{
						Name:     "Cluster 1",
						StepName: s.GetName(),
						Status:   string(StateCanceled),
					}))
			log.Infof("step %s canceled \n", s.GetName())
			wg2.Done()
			return
		case <-time.After(timeout):
			wg2.Done()
			return
		}
	}()

	var wg3 sync.WaitGroup
	var newCtx context.Context
	wg3.Add(1)
	go func() {
		select {
		case newCtx = <-ctxc:
			wg3.Done()
			return
		}
	}()
	wg1.Wait()
	wg2.Wait()
	wg3.Wait()
	close(ctxc)
	log.Infof("step %s finished waiting, Proccessing result\n", s.GetName())

	if canceledErr != nil {
		s.SetState(StateCanceled)
		log.Infof("step %s returning from cancel: %s\n", s.GetName(), canceledErr)
		return ctx, canceledErr
	}

	if s.fail {
		s.SetState(StateFailed)
		log.Infof("step %s failed\n", s.GetName())
		return ctx, errors.New(failedMsg(s.GetName()))
	}

	s.SetState(StateCompleted)
	return newCtx, nil

}

func TestTaskRunSucceed(t *testing.T) {

	taskOpts := TaskOpts{Type: TaskTypeTest, Ctx: context.Background()}

	task, _ := NewTask(taskOpts, newTestUnitStep(StepInfo{Name: "create leader", State: StateWaiting}, 2, "", Leader, false, "10.0.0.2"))

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

	err := <-result

	//waiting for steps to finish and check status
	state = task.CheckStatus()

	assert.Equal(t, state, StateCompleted)
	assert.NoError(t, err)
	//check the value is passed through context
	runtimeresult, _ := testFromContext(task.ctx)
	assert.Equal(t, runtimeresult.LeaderIP[0], "10.0.0.2")
}

func TestTaskRunFail(t *testing.T) {

	taskOpts := TaskOpts{Type: TaskTypeTest, Ctx: context.Background()}

	task, _ := NewTask(taskOpts, newTestUnitStep(StepInfo{Name: "CreateLeader", State: StateWaiting}, 0, "", Leader, true, "10.0.0.2"))

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

	err := <-result
	//waiting for steps to finish and check status
	state = task.CheckStatus()
	assert.Equal(t, state, StateFailed)
	assert.EqualError(t, err, failedMsg("CreateLeader"))
}

func TestTaskRunCancel(t *testing.T) {

	taskOpts := TaskOpts{Log: rabbit.GoLog(os.Stderr, "", 0), Type: TaskTypeTest, Ctx: context.Background()}

	task, _ := NewTask(taskOpts, newTestUnitStep(StepInfo{Name: "CreateLeader", State: StateWaiting}, 2, "", Leader, true, "10.0.0.2"))
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
	assert.Equal(t, state, StateProcessing)

	//time.Sleep(time.Second * 1)
	err := <-result
	//waiting for steps to finish and check status
	state = task.CheckStatus()
	assert.Equal(t, state, StateCanceled)
	assert.EqualError(t, err, canceledMsg("CreateLeader"))
}
