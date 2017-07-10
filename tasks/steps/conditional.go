package steps

import (
	"context"
	"fmt"
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

// Name for this step, the name of a branching step is elided
func (b *BranchingStep) Name() string {
	if b.right == nil {
		// people need to have purposely given a nil to the Then method
		// to even get here. The syntax is built up to ensure compilation fails
		// for an incomplete predicate step
		panic("a branching step needs at least a then branch defined")
	}
	if b.left == nil {
		return "~" + b.right.Name()
	}
	return fmt.Sprintf("%s|%s", b.right.Name(), b.left.Name())
}

// Announce the branches captured by this predicate step
func (b *BranchingStep) Announce(ctx context.Context) {
	PublishRegisterEvent(ctx, b.Name())
	cx := SetParentName(ctx, b.Name())
	if b.left != nil {
		b.left.Announce(cx)
	}
	b.right.Announce(cx)
}

// Run the step with the specified contest
func (b *BranchingStep) Run(ctx context.Context) (context.Context, error) {
	b.m.Lock()
	nctx := SetParentName(ctx, b.Name())
	if b.matches(ctx) {
		b.selected = b.right
		if b.left != nil {
			PublishRunEvent(nctx, b.left.Name(), StateSkipped)
		}
	} else {
		b.selected = b.left
		PublishRunEvent(nctx, b.right.Name(), StateSkipped)
	}
	b.m.Unlock()

	if b.selected == nil {
		return ctx, nil
	}

	PublishRunEvent(ctx, b.selected.Name(), StateProcessing)
	cx, err := b.selected.Run(nctx)
	if err != nil {
		if IsCanceled(err) {
			PublishRunEvent(nctx, b.selected.Name(), StateCanceled)
		} else {
			PublishRunEvent(nctx, b.selected.Name(), StateFailed)
		}
		return cx, err
	}
	PublishRunEvent(nctx, b.selected.Name(), StateSuccess)
	return cx, nil
}

// Rollback the selected step if there is one
func (b *BranchingStep) Rollback(ctx context.Context) (context.Context, error) {
	b.m.Lock()
	c := SetParentName(ctx, b.Name())
	var e error
	if b.selected != nil {
		PublishRollbackEvent(c, b.selected.Name(), StateProcessing)
		c, e = b.selected.Rollback(c)
		if e != nil {
			PublishRollbackEvent(c, b.selected.Name(), StateFailed)
		} else {
			PublishRollbackEvent(c, b.selected.Name(), StateSuccess)
		}
	}
	b.m.Unlock()
	return c, e
}
