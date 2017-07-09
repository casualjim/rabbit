package steps_test

import (
	"context"
	"testing"
	"time"

	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/casualjim/rabbit/tasks/steps/internal"
	"github.com/stretchr/testify/assert"
)

func sleepRunStep(name steps.StepName, dur time.Duration, finished chan<- time.Time) *countingStep {
	return &countingStep{
		StepName: name,
		run: func(c context.Context) (context.Context, error) {
			defer func() {
				finished <- time.Now()
				close(finished)
			}()
			select {
			case <-c.Done():
				return c, c.Err()
			case <-time.After(dur):
				return c, nil
			}
		},
	}
}

func sleepFailRunStep(name steps.StepName, dur time.Duration, finished chan<- time.Time) *countingStep {
	return &countingStep{
		StepName: name,
		run: func(c context.Context) (context.Context, error) {
			defer func() {
				finished <- time.Now()
				close(finished)
			}()
			select {
			case <-c.Done():
				return c, c.Err()
			case <-time.After(dur):
				return c, assert.AnError
			}
		},
	}
}

func TestConcurrent_Success(t *testing.T) {
	finished1 := make(chan time.Time, 1)
	finished2 := make(chan time.Time, 1)
	step1 := sleepRunStep("step1", 200*time.Millisecond, finished1)
	step2 := sleepRunStep("step2", 100*time.Millisecond, finished2)

	ctx := context.Background()

	expected := time.Now().Add(200 * time.Millisecond)
	_, err := steps.Execution(steps.ParentContext(ctx)).Run(
		steps.Concurrent(
			"concurrent-success",
			step1,
			step2,
		),
	)

	done2 := <-finished2
	done1 := <-finished1
	assert.NoError(t, err)
	assert.WithinDuration(t, expected, time.Now(), 10*time.Millisecond)
	assert.True(t, done2.Before(done1))
	assert.Equal(t, 1, step1.Runs())
	assert.Equal(t, 1, step2.Runs())
	assert.Equal(t, 0, step1.Rollbacks())
	assert.Equal(t, 0, step2.Rollbacks())
}

func TestConcurrent_Rollback(t *testing.T) {
	finished1 := make(chan time.Time, 1)
	finished2 := make(chan time.Time, 1)
	step0 := failRun("step0")
	step1 := sleepFailRunStep("step1", 200*time.Millisecond, finished1)
	step2 := sleepRunStep("step2", 100*time.Millisecond, finished2)

	ctx := context.Background()

	expected := time.Now().Add(200 * time.Millisecond)
	_, err := steps.Execution(steps.ParentContext(ctx)).Run(
		steps.Concurrent(
			"concurrent-rollback",
			step0,
			step1,
			step2,
		),
	)

	done2 := <-finished2
	done1 := <-finished1
	assert.NoError(t, err)
	assert.WithinDuration(t, expected, time.Now(), 10*time.Millisecond)
	assert.True(t, done2.Before(done1))
	assert.Equal(t, 1, step0.Runs())
	assert.Equal(t, 1, step1.Runs())
	assert.Equal(t, 1, step2.Runs())
	assert.Equal(t, 1, step0.Rollbacks())
	assert.Equal(t, 1, step1.Rollbacks())
	assert.Equal(t, 1, step2.Rollbacks())
}

func TestConcurrent_RunCancel(t *testing.T) {
	finished0 := make(chan time.Time, 1)
	finished1 := make(chan time.Time, 1)
	finished2 := make(chan time.Time, 1)
	step0 := sleepRunStep("step0", 150*time.Millisecond, finished0)
	step1 := sleepRunStep("step1", 200*time.Millisecond, finished1)
	step1.rollback = failFn
	step2 := sleepRunStep("step2", 100*time.Millisecond, finished2)

	ctx, cancel := context.WithCancel(internal.SetThrottle(context.Background(), 100*time.Millisecond))

	go func() {
		<-time.After(250 * time.Millisecond)
		cancel()
	}()
	_, err := steps.Execution(steps.ParentContext(ctx)).Run(
		steps.Concurrent(
			"concurrent-cancel",
			step0,
			step1,
			step2,
		),
	)

	<-finished1
	<-finished0
	assert.NoError(t, err)
	assert.Equal(t, 1, step0.Runs())
	assert.Equal(t, 1, step1.Runs())
	assert.Equal(t, 0, step2.Runs())
	assert.Equal(t, 1, step0.Rollbacks())
	assert.Equal(t, 1, step1.Rollbacks())
	assert.Equal(t, 0, step2.Rollbacks())
}
