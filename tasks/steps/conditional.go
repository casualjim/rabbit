package steps

import (
	"context"
	"sync"
)

// If condition for choosing
func If(pred Predicate) PredicateStep {
	return &BranchingStep{
		matches: pred,
	}
}

// PredicateStep is a partial step that exposes the Then branch in an if condition step
type PredicateStep interface {
	Then(Step) *BranchingStep
}

// BranchingStep for forking based on a condition.
// if the predicate evaluates to true then the right side will be executed
// if the predicate evaluates to false then the left side will be executed
type BranchingStep struct {
	matches  Predicate
	right    Step
	left     Step
	selected Step
	m        sync.Mutex
}

// Then step to be executed when the predicate evaluates to true
func (b *BranchingStep) Then(step Step) *BranchingStep {
	b.right = step
	return b
}

// Else step to be executed when the predicate evaluates to false
func (b *BranchingStep) Else(step Step) *BranchingStep {
	b.left = step
	return b
}

// Run the step with the specified contest
func (b *BranchingStep) Run(ctx context.Context) (context.Context, error) {
	b.m.Lock()
	if b.matches(ctx) {
		b.selected = b.right
	} else {
		b.selected = b.left
	}
	b.m.Unlock()
	return b.selected.Run(ctx)
}

// Rollback the selected step if there is one
func (b *BranchingStep) Rollback(ctx context.Context) (context.Context, error) {
	b.m.Lock()
	c := ctx
	var e error
	if b.selected != nil {
		c, e = b.selected.Rollback(c)
	}
	b.m.Unlock()
	return c, e
}
