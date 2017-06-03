///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"time"

	"github.com/casualjim/rabbit/joint"
	"github.com/go-openapi/strfmt"
	"github.com/pborman/uuid"
)

type Task struct {
	RunTime  TaskRunTime     `json:"runtime,omitempty"`
	Created  strfmt.DateTime `json:"created,omitempty"`
	ID       TaskID          `json:"id"`
	Type     TaskType        `json:"type,omitempty"`
	Name     string          `json:"name,omitempty"`
	TaskStep Step            `json:"taskstep,omitempty"`

	ctx       context.Context
	successFn func(*Task, Step)
	failFn    func(*Task, Step, *joint.Error)
}

// State is the state of a task or a step
type State string

const (
	StateNone        State = "none"
	StateCreated     State = "created"
	StateProcessing  State = "processing"
	StateCompleted   State = "completed"
	StateFailed      State = "failed"
	StateAborted     State = "aborted"
	StateRollingback State = "rollingback"
)

// TaskType type of Task
type TaskType string

// TaskID Task Id
type TaskID string

// TaskRunTime metadata to describe the Task
type TaskRunTime struct {
	// cause is for the aggregated errors
	Cause []string `json:"cause,omitempty"`
	// cluster Id
	ClusterID string `json:"clusterId,omitempty"`
	// log is for the aggregated logs
	Log []string `json:"log,omitempty"`
}

type TaskOpts struct {
	RunTime   TaskRunTime     `json:"runtime,omitempty"`
	Type      TaskType        `json:"type,omitempty"`
	Ctx       context.Context `json:"-"`
	SuccessFn func(*Task, Step)
	FailFn    func(*Task, Step, *joint.Error)
}

func NewTask(taskOpts TaskOpts, step Step) (*Task, error) {
	return &Task{
		RunTime:   taskOpts.RunTime,
		Created:   strfmt.DateTime(time.Now().UTC()),
		ID:        TaskID(uuid.New()),
		Type:      taskOpts.Type,
		TaskStep:  step,
		ctx:       taskOpts.Ctx,
		successFn: taskOpts.SuccessFn,
		failFn:    taskOpts.FailFn,
	}, nil
}

func (t *Task) Run() error {
	var e *joint.Error
	//create a new context to include runtime as value so that step can write to it
	t.ctx = NewContext(t.ctx, t.RunTime)

	//execute step.Run() and update context
	t.ctx, e = t.TaskStep.Run(t.ctx)

	//update runtime from context value
	runtime, _ := FromContext(t.ctx)
	t.RunTime = runtime

	if e != nil {
		t.failFn(t, t.TaskStep, e)
		return e
	}

	t.successFn(t, t.TaskStep)

	return nil
}

func (t *Task) Rollback() error {
	return nil
}

func (t *Task) CheckStatus() State {
	info := t.TaskStep.GetInfo()
	if info.State == StateNone {
		return StateCreated
	}
	return info.State
}

func (t *Task) GetName() string {
	return t.Name
}

func (t *Task) GetID() string {
	return string(t.ID)
}

///////////////////////////////////////////////////////////////////////
//Utility functions for Task
type key int

const runtimeKey key = 0

func NewContext(ctx context.Context, runtime TaskRunTime) context.Context {
	return context.WithValue(ctx, runtimeKey, runtime)
}

func FromContext(ctx context.Context) (TaskRunTime, bool) {
	// ctx.Value returns nil if ctx has no value for the key;
	runtime, ok := ctx.Value(runtimeKey).(TaskRunTime)
	return runtime, ok
}
