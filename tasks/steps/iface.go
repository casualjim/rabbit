package steps

import "context"

// A Publisher knows how to publish events
type Publisher interface {
	Publish(interface{})
}

// Registerable items know how to register themselves in a parent
type Registerable interface {
	Register(string, Publisher)
}

// A Decider for determining to roll back or not
type Decider func(error) bool

// A Step encapsulates a unit of work.
type Step interface {
	Run(context.Context) (context.Context, error)
	Rollback(context.Context) (context.Context, error)
}

// Run handler for a step
type Run func(context.Context) (context.Context, error)

// Rollback handler for a step
type Rollback func(context.Context) (context.Context, error)

// Predicate for branching execution left or right
type Predicate func(context.Context) bool
