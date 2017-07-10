package steps

import "context"

// A Decider for determining to roll back or not
type Decider func(error) bool

// Predicate for branching execution left or right
type Predicate func(context.Context) bool

// A Step encapsulates a unit of work.
type Step interface {
	Name() string
	Announce(context.Context)
	Run(context.Context) (context.Context, error)
	Rollback(context.Context) (context.Context, error)
}
