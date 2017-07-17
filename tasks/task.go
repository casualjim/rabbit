package tasks

import (
	"context"
	"os"
	"sync"

	"github.com/casualjim/rabbit"
	"github.com/casualjim/rabbit/eventbus"
	"github.com/casualjim/rabbit/tasks/rollback"
	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/segmentio/ksuid"
)

// A Task encapsulates the execution of a step and provides context and configuration.
// It serves as an executor and main interface for the user of the library.
// Steps assume they are being run by a task.
type Task interface {
	ID() string
	Run() error
	Cancel()
	Subscribe(...eventbus.EventHandler) Task
	Unsubscribe(...eventbus.EventHandler) Task
}

// TaskOpt represents an option for a task
type TaskOpt func(*task)

// ParentContext provides a parent context for the task
func ParentContext(ctx context.Context) TaskOpt {
	return func(t *task) { t.ctx = ctx }
}

// Run the provided step when run is called
func Run(step steps.Step) TaskOpt {
	return func(t *task) { t.step = step }
}

// Should the step fail then use the provided rollback stratgey
func Should(rollback steps.Decider) TaskOpt {
	return func(t *task) { t.decider = rollback }
}

// LogWith is used to log warning and error messages in a task.
//
// There are very few usages of this, but when an error is not returned but seen
// we will use this logger to log failed closes and the like.
// So it is advisable to provide one, the default option is to log to /dev/null
func LogWith(log rabbit.Logger) TaskOpt {
	return func(t *task) { t.log = log }
}

// Create a new task
func Create(opts ...TaskOpt) Task {
	id := ksuid.New()
	tsk := &task{
		id:             id,
		ctx:            context.Background(),
		bus:            eventbus.New(rabbit.GoLog(os.Stderr, "["+id.String()+"] ", 0)),
		runStates:      &stateStore{states: make(map[string]steps.State)},
		rollbackStates: &stateStore{states: make(map[string]steps.State)},
		decider:        rollback.Always,
		log:            rabbit.NopLogger,
	}

	for _, opt := range opts {
		opt(tsk)
	}

	tsk.bus.Subscribe(
		eventbus.Handler(tsk.trackStepStates),
	)

	tsk.plan = steps.Plan(
		steps.PublishTo(tsk.bus),
		steps.ParentContext(tsk.ctx),
		steps.Should(tsk.decider),
		steps.Run(tsk.step),
	)

	return tsk
}

type task struct {
	id      ksuid.KSUID
	bus     eventbus.EventBus
	ctx     context.Context
	step    steps.Step
	decider steps.Decider
	plan    *steps.Planned
	log     rabbit.Logger

	runStates      *stateStore
	rollbackStates *stateStore
}

func (t *task) trackStepStates(evt eventbus.Event) error {
	return nil
}

func (t *task) ID() string {
	return t.id.String()
}
func (t *task) Run() error {
	ctx, err := t.plan.Execute()
	t.ctx = ctx
	if err != nil {
		t.bus.Close()
		return err
	}
	if err := t.bus.Close(); err != nil {

	}
	return nil
}

func (t *task) Cancel() {
	t.plan.Cancel()
}

func (t *task) Subscribe(handlers ...eventbus.EventHandler) Task {
	t.bus.Subscribe(handlers...)
	return t
}

func (t *task) Unsubscribe(handlers ...eventbus.EventHandler) Task {
	t.bus.Unsubscribe(handlers...)
	return t
}

type stateStore struct {
	m      sync.RWMutex
	states map[string]steps.State
}

func (s *stateStore) Get(key string) steps.State {
	s.m.RLock()
	v := s.states[key]
	s.m.RUnlock()
	return v
}

func (s *stateStore) GetOK(key string) (steps.State, bool) {
	s.m.RLock()
	v, ok := s.states[key]
	s.m.RUnlock()
	return v, ok
}

func (s *stateStore) Delete(key string) {
	s.m.Lock()
	delete(s.states, key)
	s.m.Unlock()
}

func (s *stateStore) Set(key string, state steps.State) {
	s.m.Lock()
	s.states[key] = state
	s.m.Unlock()
}
