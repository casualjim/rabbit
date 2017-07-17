package steps_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/casualjim/rabbit/eventbus"
	"github.com/casualjim/rabbit/tasks/internal"
	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/stretchr/testify/assert"
)

func TestStepStates(t *testing.T) {
	var allStates = []struct {
		Key  steps.State
		Name string
	}{
		{steps.StateUnknown, "unknown"},
		{steps.StateWaiting, "waiting"},
		{steps.StateSkipped, "skipped"},
		{steps.StateProcessing, "processing"},
		{steps.StateSuccess, "completed"},
		{steps.StateFailed, "failed"},
		{steps.StateCanceled, "canceled"},
	}

	for _, v := range allStates {
		st, err := steps.StateFromString(v.Name)
		if assert.NoError(t, err) {
			assert.Equal(t, v.Key, st)
		}
		assert.Equal(t, v.Name, v.Key.String())
		b, _ := json.Marshal(v.Key)
		assert.Equal(t, fmt.Sprintf("%q", v.Name), string(b))
		var k steps.State
		json.Unmarshal(b, &k)
		assert.Equal(t, v.Key, k)
	}

	st, err := steps.StateFromString("blah")
	if assert.Error(t, err) {
		assert.Equal(t, steps.StateUnknown, st)
	}
	var k steps.State
	assert.Error(t, json.Unmarshal([]byte("\"blah\""), &k))
}

func TestStepActions(t *testing.T) {
	var allActions = []struct {
		Key  steps.Action
		Name string
	}{
		{steps.ActionInit, "init"},
		{steps.ActionRun, "run"},
		{steps.ActionRollback, "rollback"},
	}

	for _, v := range allActions {
		st, err := steps.ActionFromString(v.Name)
		if assert.NoError(t, err) {
			assert.Equal(t, v.Key, st)
		}
		assert.Equal(t, v.Name, v.Key.String())
		b, _ := json.Marshal(v.Key)
		assert.Equal(t, fmt.Sprintf("%q", v.Name), string(b))
		var k steps.Action
		json.Unmarshal(b, &k)
		assert.Equal(t, v.Key, k)
	}

	st, err := steps.ActionFromString("blah")
	if assert.Error(t, err) {
		assert.Equal(t, steps.ActionInit, st)
	}
	var k steps.Action
	assert.Error(t, json.Unmarshal([]byte("\"blah\""), &k))
}

func TestIsLifecycle(t *testing.T) {
	bogus := struct{}{}
	evt := eventbus.Event{
		Name: "bogus",
		At:   time.Now(),
		Args: bogus,
	}
	assert.False(t, steps.IsLifecycleEvent(evt, steps.ActionRun, steps.StateSkipped))
}

func TestRetryEventFilter(t *testing.T) {
	matches := eventbus.Event{
		Name: steps.TopicRetry,
		At:   time.Now(),
		Args: steps.RetryEvent{
			Name:   "someStep",
			Next:   10 * time.Second,
			Parent: "some parent",
			Reason: assert.AnError,
		},
	}

	assert.True(t, steps.RetryEventFilter(matches))
	assert.False(t, steps.RetryEventFilter(eventbus.Event{
		Name: "bogus",
		At:   time.Now(),
		Args: struct{}{},
	}))
	assert.False(t, steps.RetryEventFilter(eventbus.Event{
		Name: steps.TopicRetry,
		At:   time.Now(),
		Args: struct{}{},
	}))
}

func TestPublishApplicationEvent(t *testing.T) {
	bus := testBus()
	ctx := internal.SetPublisher(context.Background(), bus)

	type blah int
	var i blah = 1
	steps.PublishEvent(ctx, i)
	cnt := bus.Count(steps.IsApplicationEvent)

	assert.Equal(t, 1, cnt)
}
