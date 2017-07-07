package step

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
