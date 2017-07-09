package internal

import (
	"context"
	"time"

	"github.com/casualjim/rabbit/eventbus"
)

// StepKey are keys use in steps
type StepKey uint8

const (
	// PublisherKey for the publisher in the context
	PublisherKey StepKey = iota
	// LoggerKey for the log entry in the context
	LoggerKey
	// ParentNameKey for the name of the parent
	ParentNameKey
	// TaskNameKey for the name of the task
	TaskNameKey
	// StepNameKey for the name of the step (full path)
	StepNameKey

	// TestThrottle is only use for testing
	testThrottleKey StepKey = 255
)

// SetThrottle on the context for use in tests
func SetThrottle(ctx context.Context, dur time.Duration) context.Context {
	return context.WithValue(ctx, testThrottleKey, dur)
}

// GetThrottle from the context for use in tests
func GetThrottle(ctx context.Context) time.Duration {
	dur, ok := ctx.Value(testThrottleKey).(time.Duration)
	if !ok {
		return 0
	}
	return dur
}

// SetPublisher on the context
func SetPublisher(ctx context.Context, pub eventbus.EventBus) context.Context {
	return context.WithValue(ctx, PublisherKey, pub)
}

// GetPublisher from the context
func GetPublisher(ctx context.Context) eventbus.EventBus {
	bus, ok := ctx.Value(PublisherKey).(eventbus.EventBus)
	if !ok {
		return nil
	}
	return bus
}