package tasks

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/casualjim/rabbit"
	"github.com/casualjim/rabbit/eventbus"
	"github.com/casualjim/rabbit/tasks/rollback"
	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/go-openapi/strfmt"
	"github.com/segmentio/ksuid"
)

// A Task encapsulates the execution of a step and provides context and configuration.
// It serves as an executor and main interface for the user of the library.
// Steps assume they are being run by a task.
type Task interface {
	ID() string
	Name() string
	CreatedAt() time.Time
	FinishedAt() time.Time
	FirstInfo(string) (StepInfo, bool)
	Infos(string) []StepInfo

	Run() error
	// Cancel allows cancelling the request with a new decider
	// which in turn allows for getting abort or rollback behavior
	Cancel(steps.Decider)
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
func Create(name string, opts ...TaskOpt) Task {
	id := ksuid.New()
	tsk := &task{
		id:        id,
		name:      name,
		createdAt: time.Now(),
		ctx:       context.Background(),
		bus:       eventbus.New(rabbit.GoLog(os.Stderr, "["+id.String()+"] ", 0)),
		decider:   rollback.Always,
		log:       rabbit.NopLogger,
	}

	for _, opt := range opts {
		opt(tsk)
	}

	tsk.plan = steps.Plan(
		steps.PublishTo(tsk.bus),
		steps.ParentContext(tsk.ctx),
		steps.Should(tsk.decider),
		steps.Run(tsk.step),
	)
	tsk.bus.Subscribe(
		eventbus.Handler(tsk.trackStepStates),
	)
	tsk.states = newStateStore(tsk.plan.StepNames())

	return tsk
}

type task struct {
	id         ksuid.KSUID
	name       string
	createdAt  time.Time
	finishedat time.Time
	bus        eventbus.EventBus
	ctx        context.Context
	step       steps.Step
	decider    steps.Decider
	plan       *steps.Planned
	log        rabbit.Logger
	states     *stateStore
}

// func (t task) MarshalJSON() ([]byte, error) {
// 	var tsk TaskInfo
// 	tsk.ID = t.id.String()
// 	tsk.Name = t.name
// 	tsk.CreatedAt = strfmt.DateTime(t.createdAt)

// 	if !t.finishedat.IsZero() {
// 		tsk.FinishedAt = strfmt.DateTime(t.finishedat)
// 	}
// 	steps := t.states.Infos("")
// 	if len(steps) > 0 {
// 		root := steps[0]
// 		tsk.Phase = root.Phase
// 		tsk.State = root.State
// 		tsk.Steps = steps[1:]
// 	}
// 	return json.Marshal(tsk)
// }

// func (t *task) UnmarshalJSON(data []byte) error {
// 	var tsk TaskInfo
// 	if err := json.Unmarshal(data, &tsk); err != nil {
// 		return err
// 	}
// 	t.id = tsk.ID
// 	t.Name = tsk.Name
// 	t.createdAt = tsk.CreatedAt
// 	t.finishedat = tsk.FinishedAt

// }

func (t *task) Name() string {
	return t.name
}

func (t *task) CreatedAt() time.Time {
	return t.createdAt
}

func (t *task) FinishedAt() time.Time {
	return t.finishedat
}

func (t *task) FirstInfo(key string) (StepInfo, bool) {
	return t.states.FirstInfo(key)
}

func (t *task) Infos(key string) []StepInfo {
	return t.states.Infos(key)
}

func (t *task) trackStepStates(evt eventbus.Event) error {
	switch evt.Name {
	case steps.TopicLifecycle:
		if et, ok := evt.Args.(steps.LifecycleEvent); ok && et.Action != steps.ActionInit {
			t.states.AddLifecycleEvent(et)
		}
	case steps.TopicRetry:
		if et, ok := evt.Args.(steps.RetryEvent); ok {
			t.states.AddRetryEvent(et)
		}
	}
	return nil
}

func (t *task) ID() string {
	return t.id.String()
}
func (t *task) Run() error {
	ctx, err := t.plan.Execute()
	t.ctx = ctx
	if err != nil {
		if e2 := t.bus.Close(); e2 != nil {
			t.log.Warnf("failed to close eventbus: %v", e2)
		}
		t.finishedat = time.Now()
		return err
	}
	if err := t.bus.Close(); err != nil {
		t.log.Warnf("failed to close eventbus: %v", err)
	}
	t.finishedat = time.Now()
	return nil
}

func (t *task) Cancel(decider steps.Decider) {
	t.plan.Cancel(decider)
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
	m         sync.RWMutex
	states    map[string]StepInfo
	stepNames []string
}

func newStateStore(stepNames []string) *stateStore {
	store := &stateStore{
		states:    make(map[string]StepInfo, 150),
		stepNames: stepNames,
	}

	store.AppendStepNames(stepNames)
	return store
}

func (s *stateStore) AppendStepNames(stepNames []string) {
	s.m.Lock()
	for _, stepName := range stepNames {
		if _, ok := s.states[stepName]; !ok {
			parent, name := s.splitPath(stepName)
			s.states[stepName] = StepInfo{
				Name:   name,
				Parent: parent,
				Path:   stepName,
				Phase:  steps.ActionInit,
				State:  steps.StateWaiting,
			}
			s.stepNames = append(s.stepNames, stepName)
		}
	}
	s.m.Unlock()
}

func (s *stateStore) AddLifecycleEvent(evt steps.LifecycleEvent) {
	s.m.Lock()
	path := s.fullName(evt.Parent, evt.Name)
	if info, ok := s.states[path]; ok {
		info.Phase = evt.Action
		info.State = evt.State
		if evt.State == steps.StateFailed {
			info.Reason = evt.Reason
		}
		info.NextRetry = 0
		s.states[path] = info
	}
	s.m.Unlock()
}

func (s *stateStore) AddRetryEvent(evt steps.RetryEvent) {
	s.m.Lock()
	path := s.fullName(evt.Parent, evt.Name)
	if info, ok := s.states[path]; ok {
		info.Retry = append(info.Retry, evt.Reason)
		info.NextRetry = evt.Next
		s.states[path] = info
	}
	s.m.Unlock()
}

func (s *stateStore) fullName(parent, name string) string {
	if parent == "" {
		return name
	}
	return fmt.Sprintf("%s.%s", parent, name)
}

func (s *stateStore) splitPath(path string) (parent string, name string) {
	parts := strings.Split(path, ".")
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

func (s *stateStore) FirstInfo(key string) (StepInfo, bool) {
	s.m.RLock()
	if key == "" && len(s.stepNames) > 0 {
		key = s.stepNames[0]
	}
	info, ok := s.states[key]
	s.m.RUnlock()
	return info, ok
}

func (s *stateStore) Infos(key string) []StepInfo {
	s.m.RLock()
	var result []StepInfo
	for _, sn := range s.stepNames {
		if key == "" || strings.HasPrefix(sn, key) {
			result = append(result, s.states[sn])
		}
	}
	s.m.RUnlock()
	return result
}

// StepInfo contains the information about a step
type StepInfo struct {
	Name      string        `json:"name"`
	Phase     steps.Action  `json:"phase"`
	State     steps.State   `json:"state"`
	Path      string        `json:"path"`
	Parent    string        `json:"parent,omitempty"`
	Reason    error         `json:"reason,omitempty"`
	Retry     []error       `json:"retry,omitempty"`
	NextRetry time.Duration `json:"nextRetry,omitempty"`
}

// TaskInfo contains the information about a task
type TaskInfo struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	CreatedAt  strfmt.DateTime `json:"createdAt"`
	FinishedAt strfmt.DateTime `json:"finishedAt,omitempty"`
	Phase      steps.Action    `json:"phase"`
	State      steps.State     `json:"state"`
	Steps      OrderedSteps    `json:"steps"`
}

// OrderedSteps represents an ordered tree of steps as a json object
type OrderedSteps []KeyValue

// func (o OrderedSteps) MarshalJSON() ([]byte, error) {
// 	var w jwriter.Writer
// 	w.RawByte('{')
// 	for i, kv := range o {
// 		if i > 0 {
// 			w.RawByte(',')
// 		}
// 		w.RawString("\"" + kv.Key + "\":")
// 		w.RawByte(':')
// 		w.Raw(json.Marshal(kv.Value))
// 	}
// 	w.RawByte('}')
// 	return w.BuildBytes()
// }

type KeyValue struct {
	Key   string
	Value StepInfo
}

// func (k KeyValue) MarshalJSON() ([]byte, error) {
// 	return json.Marshal(map[string]StepInfo{k.Key: k.Value})
// }

// func (kv *KeyValue) UnmarshalJSON(data []byte) error {
// 	d := make(map[string]StepInfo, 1)
// 	if err := json.Unmarshal(data, &d); err != nil {
// 		return err
// 	}

// 	var n KeyValue
// 	for k, v := range d {
// 		n.Key = k
// 		n.Value = v
// 	}
// 	*kv = n
// 	return nil
// }
