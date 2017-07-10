package steps_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/casualjim/rabbit/eventbus"
	"github.com/casualjim/rabbit/tasks/rollback"
	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/stretchr/testify/assert"
)

func failFn(c context.Context) (context.Context, error)     { return c, assert.AnError }
func noopFn(c context.Context) (context.Context, error)     { return c, nil }
func canceledFn(c context.Context) (context.Context, error) { return c, context.Canceled }

func failRun(name steps.StepName) *countingStep {
	return stepRun(name, failFn)
}

func stepRun(name steps.StepName, fn func(context.Context) (context.Context, error)) *countingStep {
	return &countingStep{StepName: name, run: fn}
}

type countingStep struct {
	steps.StepName
	run           func(context.Context) (context.Context, error)
	rollback      func(context.Context) (context.Context, error)
	runCount      int64
	rollbackCount int64
}

func (c *countingStep) Run(ctx context.Context) (context.Context, error) {
	atomic.AddInt64(&c.runCount, 1)
	if c.run != nil {
		return c.run(ctx)
	}
	return ctx, nil
}

func (c *countingStep) Rollback(ctx context.Context) (context.Context, error) {
	atomic.AddInt64(&c.rollbackCount, 1)
	if c.rollback != nil {
		return c.rollback(ctx)
	}
	return ctx, nil
}

func (c *countingStep) Runs() int {
	return int(atomic.LoadInt64(&c.runCount))
}

func (c *countingStep) Rollbacks() int {
	return int(atomic.LoadInt64(&c.rollbackCount))
}

func TestExecutor_Run(t *testing.T) {
	rs := &countingStep{StepName: steps.StepName("run")}

	assert.NotPanics(t, (&(steps.Planned{})).Cancel)

	exec := steps.Plan(
		steps.Should(rollback.Always),
		steps.PublishTo(eventbus.NopBus),
		steps.Run(rs),
	)
	ctx := exec.Context()
	cx, err := exec.Execute()

	if assert.NoError(t, err) {
		assert.Equal(t, ctx, cx)
		assert.Equal(t, 1, rs.Runs())
		assert.Equal(t, 0, rs.Rollbacks())
	}
}

func TestExecutor_Rollback(t *testing.T) {
	rs := failRun("rollback")

	executor := steps.Plan(
		steps.Should(rollback.Always),
		steps.PublishTo(eventbus.NopBus),
		steps.Run(rs),
	)
	ctx := executor.Context()
	cx, err := executor.Execute()
	if assert.NoError(t, err) {
		assert.Equal(t, ctx, cx)
		assert.Equal(t, 1, rs.Runs())
		assert.Equal(t, 1, rs.Rollbacks())
	}
}

func TestAnnounce_Success(t *testing.T) {
	bus := testBus()

	rs := stepRun("the-step", nil)

	steps.Plan(
		steps.PublishTo(bus),
		steps.Run(rs),
	).Execute()

	bus.Assert(t, eventCounts{
		Registered: 1,
		RunRunning: 1,
		RunSuccess: 1,
	})
}

func TestAnnounce_Canceled(t *testing.T) {
	bus := testBus()

	rs := stepRun("the-step", canceledFn)

	steps.Plan(
		steps.PublishTo(bus),
		steps.Run(rs),
	).Execute()

	bus.Assert(t, eventCounts{
		Registered:  1,
		RunRunning:  1,
		RunCanceled: 1,
		RbRunning:   1,
		RbSuccess:   1,
	})
}

func TestAnnounce_Failed(t *testing.T) {
	bus := testBus()

	rs := failRun("the-step")

	steps.Plan(
		steps.PublishTo(bus),
		steps.Run(rs),
	).Execute()

	bus.Assert(t, eventCounts{
		Registered: 1,
		RunRunning: 1,
		RunFailed:  1,
		RbRunning:  1,
		RbSuccess:  1,
	})
}
func TestAnnounce_RollbackSkipped(t *testing.T) {
	bus := testBus()

	rs := failRun("the-step")

	steps.Plan(
		steps.PublishTo(bus),
		steps.Should(rollback.Never),
		steps.Run(rs),
	).Execute()

	bus.Assert(t, eventCounts{
		Registered: 1,
		RunRunning: 1,
		RunFailed:  1,
		RbSkipped:  1,
	})
}
func TestAnnounce_RollbackFailed(t *testing.T) {
	bus := testBus()

	rs := failRun("the-step")
	rs.rollback = failFn

	steps.Plan(
		steps.PublishTo(bus),
		steps.Run(rs),
	).Execute()

	bus.Assert(t, eventCounts{
		Registered: 1,
		RunRunning: 1,
		RunFailed:  1,
		RbRunning:  1,
		RbFailed:   1,
	})
}

func testBus() *aggregatingBus {
	return &aggregatingBus{}
}

type aggregatingBus struct {
	mw   sync.Mutex
	evts []eventbus.Event
}

func (a *aggregatingBus) Close() error {
	return nil
}
func (a *aggregatingBus) Publish(evt eventbus.Event) {
	a.mw.Lock()
	a.evts = append(a.evts, evt)
	a.mw.Unlock()
}
func (a *aggregatingBus) Subscribe(...eventbus.EventHandler)   {}
func (a *aggregatingBus) Unsubscribe(...eventbus.EventHandler) {}
func (a *aggregatingBus) Len() int {
	return 1
}

func (a *aggregatingBus) Count(filter eventbus.EventPredicate) (cnt int) {
	for _, evt := range a.evts {
		if filter(evt) {
			cnt++
		}
	}
	return
}

func (a *aggregatingBus) Filter(filter eventbus.EventPredicate) (result []eventbus.Event) {
	for _, evt := range a.evts {
		if filter(evt) {
			result = append(result, evt)
		}
	}
	return
}

func (a *aggregatingBus) Assert(t testing.TB, c eventCounts) bool {
	init := assert.Equal(t, c.Registered, a.Count(steps.LifecycleEventFilter(steps.ActionInit, steps.StateWaiting)), "registered count")
	runSk := assert.Equal(t, c.RunSkipped, a.Count(steps.LifecycleEventFilter(steps.ActionRun, steps.StateSkipped)), "run skipped count")
	runRun := assert.Equal(t, c.RunRunning, a.Count(steps.LifecycleEventFilter(steps.ActionRun, steps.StateProcessing)), "run running count")
	runOK := assert.Equal(t, c.RunSuccess, a.Count(steps.LifecycleEventFilter(steps.ActionRun, steps.StateSuccess)), "run success count")
	runFail := assert.Equal(t, c.RunFailed, a.Count(steps.LifecycleEventFilter(steps.ActionRun, steps.StateFailed)), "run failed count")
	runCanc := assert.Equal(t, c.RunCanceled, a.Count(steps.LifecycleEventFilter(steps.ActionRun, steps.StateCanceled)), "run canceled count")
	rbRun := assert.Equal(t, c.RbRunning, a.Count(steps.LifecycleEventFilter(steps.ActionRollback, steps.StateProcessing)), "rollback running count")
	rbFail := assert.Equal(t, c.RbFailed, a.Count(steps.LifecycleEventFilter(steps.ActionRollback, steps.StateFailed)), "rollback failed count")
	rbOK := assert.Equal(t, c.RbSuccess, a.Count(steps.LifecycleEventFilter(steps.ActionRollback, steps.StateSuccess)), "rollback success count")
	rbSkip := assert.Equal(t, c.RbSkipped, a.Count(steps.LifecycleEventFilter(steps.ActionRollback, steps.StateSkipped)), "rollback skipped count")

	return init && runSk && runRun && runOK && runFail && runCanc && rbRun && rbFail && rbSkip && rbOK
}

type eventCounts struct {
	Registered  int
	RunSkipped  int
	RunRunning  int
	RunSuccess  int
	RunFailed   int
	RunCanceled int
	RbRunning   int
	RbSkipped   int
	RbSuccess   int
	RbFailed    int
}
