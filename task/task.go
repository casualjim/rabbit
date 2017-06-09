///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////

package task

import (
	"context"
	"sync"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/pborman/uuid"
)

type Task struct {
	Created  strfmt.DateTime `json:"created,omitempty"`
	ID       TaskID          `json:"id"`
	Type     TaskType        `json:"type,omitempty"`
	Name     string          `json:"name,omitempty"`
	TaskStep Step            `json:"taskstep,omitempty"`

	ctx       context.Context
	successFn func(*Task)
	failFn    func(*Task, error)
}

// State is the state of a task or a step
type State string

const (
	StateNone        State = "none"
	StateCreated     State = "created"
	StateProcessing  State = "processing"
	StateCompleted   State = "completed"
	StateFailed      State = "failed"
	StateCanceled    State = "canceled"
	StateRollingback State = "rollingback"
)

// TaskType type of Task
type TaskType string

// TaskID Task Id
type TaskID string

type TaskOpts struct {
	Type      TaskType        `json:"type,omitempty"`
	Ctx       context.Context `json:"-"`
	SuccessFn func(*Task)
	FailFn    func(*Task, error)
}

func NewTask(taskOpts TaskOpts, step Step) (*Task, error) {
	return &Task{
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
	var err error
	reqCtx, cancel := context.WithCancel(t.ctx)
	t.ctx = reqCtx
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		t.ctx, err = t.TaskStep.Run(t.ctx)

		select {
		case <-t.ctx.Done():
			err = t.Rollback(err)
			wg.Done()
			return
		default:
		}
		wg.Done()
	}()
	wg.Wait()

	if err == nil {
		t.Success()
	} else {
		t.Fail(err)
	}

	return err
}

func (t *Task) Rollback(err error) error {
	return err
}

//CheckStatus gives the state of the task's step
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

func (t *Task) Success() {
	if t.successFn == nil {
		return
	}
	t.successFn(t)
}

func (t *Task) Fail(err error) {
	if t.failFn == nil {
		return
	}
	t.failFn(t, err)
}
