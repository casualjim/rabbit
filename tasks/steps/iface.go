package steps

import "context"

// A Decider for determining to roll back or not
type Decider func(error) bool

// A Step encapsulates a unit of work.
type Step interface {
	Name() string
	Run(context.Context) (context.Context, error)
	Rollback(context.Context) (context.Context, error)
}

// Run handler for a step
type Run func(context.Context) (context.Context, error)

// Rollback handler for a step
type Rollback func(context.Context) (context.Context, error)

// Predicate for branching execution left or right
type Predicate func(context.Context) bool

// Logger interface for use in steps
type Logger interface {
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Errorf(string, ...interface{})
}
