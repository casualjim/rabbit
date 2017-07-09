package internal

import (
	"context"
	"time"

	"github.com/casualjim/rabbit/eventbus"
)

// SetPublisher on the context
func SetPublisher(ctx context.Context, pub eventbus.EventBus) context.Context {
	return context.WithValue(ctx, PublisherKey, pub)
}

// GetPublisher from the context
func GetPublisher(ctx context.Context) eventbus.EventBus {
	bus, ok := ctx.Value(PublisherKey).(eventbus.EventBus)
	if !ok {
		return eventbus.NopBus
	}
	return bus
}

// PublishEvent publishes an event to the context
func PublishEvent(ctx context.Context, name string, args interface{}) {
	pub := GetPublisher(ctx)
	pub.Publish(eventbus.Event{
		Name: name,
		At:   time.Now(),
		Args: args,
	})
}
