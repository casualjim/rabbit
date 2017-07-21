package steps

import (
	"context"
	"fmt"
	"time"

	"github.com/casualjim/rabbit/eventbus"
	"github.com/casualjim/rabbit/tasks/internal"
)

var stateKeyNames map[State]string
var namedStateKeys map[string]State

func init() {
	stateKeyNames = map[State]string{
		StateUnknown:    "unknown",
		StateWaiting:    "waiting",
		StateSkipped:    "skipped",
		StateProcessing: "processing",
		StateSuccess:    "completed",
		StateFailed:     "failed",
		StateCanceled:   "canceled",
	}

	namedStateKeys = make(map[string]State, len(stateKeyNames))
	for k, v := range stateKeyNames {
		namedStateKeys[v] = k
	}
}

// StateFromString creates a step state from a string
func StateFromString(name string) (State, error) {
	if v, ok := namedStateKeys[name]; ok {
		return v, nil
	}
	return StateUnknown, fmt.Errorf("invalid step state %q", name)
}

// State represents an event name
type State uint8

const (
	// StateUnknown indicates the step is unknown
	StateUnknown State = iota
	// StateWaiting indicates the step is known but hasn't started yet
	StateWaiting
	// StateProcessing indicates the step is currently executing
	StateProcessing
	// StateSkipped indicates the step has been skipped
	StateSkipped
	// StateSuccess indicates the step was executed successfully
	StateSuccess
	// StateFailed indicates the step has failed
	StateFailed
	// StateCanceled indicates the step was canceled
	StateCanceled
)

func (e State) String() string {
	return stateKeyNames[e]
}

// MarshalText renders this stepstate to text
func (e State) MarshalText() (text []byte, err error) {
	return []byte(stateKeyNames[e]), nil
}

// UnmarshalText parses this step state from text
func (e *State) UnmarshalText(text []byte) error {
	st, err := StateFromString(string(text))
	if err != nil {
		return err
	}
	*e = st
	return nil
}

var actionKeyNames map[Action]string
var namedActionKeys map[string]Action

func init() {
	actionKeyNames = map[Action]string{
		ActionInit:     "init",
		ActionRun:      "run",
		ActionRollback: "rollback",
	}

	namedActionKeys = make(map[string]Action, len(actionKeyNames))
	for k, v := range actionKeyNames {
		namedActionKeys[v] = k
	}
}

// ActionFromString creates a step state from a string
func ActionFromString(name string) (Action, error) {
	if v, ok := namedActionKeys[name]; ok {
		return v, nil
	}
	return ActionInit, fmt.Errorf("invalid step action %q", name)
}

// Action indicates the phase of the lifecycle the event is currently in
type Action uint8

const (
	// ActionInit is emitted when the step hasn't run yet
	ActionInit Action = iota
	// ActionRun is emitted when the step is running
	ActionRun
	// ActionRollback is emitted when the step is rolling back
	ActionRollback
)

func (e Action) String() string {
	return actionKeyNames[e]
}

// MarshalText renders this stepstate to text
func (e Action) MarshalText() (text []byte, err error) {
	return []byte(actionKeyNames[e]), nil
}

// UnmarshalText parses this step state from text
func (e *Action) UnmarshalText(text []byte) error {
	a, err := ActionFromString(string(text))
	if err != nil {
		return err
	}
	*e = a
	return nil
}

const (
	// TopicLifecycle is the event topic for lifecycle events
	TopicLifecycle = "lifecycle"
	// TopicRetry is the event topic for retries
	TopicRetry = "retry"

	// TopicApplication is the event topic for application specific events
	TopicApplication = "application"
)

// RetryEvent is emitted when a retry is executed
type RetryEvent struct {
	Name   string
	Parent string
	Reason error
	Next   time.Duration
}

// A LifecycleEvent is emitted for lifecycle operations on a step
type LifecycleEvent struct {
	Action Action
	State  State
	Name   string
	Parent string
	Reason error
}

// PublishRegisterEvent during initialization to make the step known
func PublishRegisterEvent(ctx context.Context, stepName string) {
	internal.PublishEvent(ctx, TopicLifecycle, LifecycleEvent{
		Action: ActionInit,
		State:  StateWaiting,
		Name:   stepName,
		Parent: GetParentName(ctx),
	})
}

// PublishRunEvent for state transitions during the run phase of a step
func PublishRunEvent(ctx context.Context, stepName string, state State, reason error) {
	internal.PublishEvent(ctx, TopicLifecycle, LifecycleEvent{
		Action: ActionRun,
		State:  state,
		Name:   stepName,
		Parent: GetParentName(ctx),
		Reason: reason,
	})
}

// PublishRollbackEvent for state transitions during the run phase of a step
func PublishRollbackEvent(ctx context.Context, stepName string, state State, reason error) {
	internal.PublishEvent(ctx, TopicLifecycle, LifecycleEvent{
		Action: ActionRollback,
		State:  state,
		Name:   stepName,
		Parent: GetParentName(ctx),
		Reason: reason,
	})
}

// IsLifecycleEvent returns true if this is a lifecycle event for the given action and given state
func IsLifecycleEvent(evt eventbus.Event, action Action, state State) bool {
	return LifecycleEventFilter(action, state)(evt)
}

// LifecycleEventFilter is an event filter that matches specific lifecycle events
func LifecycleEventFilter(action Action, state State) eventbus.EventPredicate {
	return func(evt eventbus.Event) bool {
		if evt.Name != TopicLifecycle {
			return false
		}
		lce, ok := evt.Args.(LifecycleEvent)
		return ok && lce.State == state && lce.Action == action
	}
}

// RetryEventFilter an event handler filter that only selects retry events
func RetryEventFilter(evt eventbus.Event) bool {
	if evt.Name != TopicRetry {
		return false
	}
	_, ok := evt.Args.(RetryEvent)
	return ok
}

// PublishEvent publishes application specific events
func PublishEvent(ctx context.Context, args interface{}) {
	internal.PublishEvent(ctx, TopicApplication, args)
}

// IsApplicationEvent returns true when the event is an application event
func IsApplicationEvent(evt eventbus.Event) bool {
	return evt.Name == TopicApplication
}
