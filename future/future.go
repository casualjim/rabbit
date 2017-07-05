package future

import (
	"context"
	"sync"
)

// Value to be returned from a future
type Value interface{}

type result struct {
	Value Value
	Err   error
	Ctx   context.Context
	_     struct{} // avoid unkeyed usage
}

// Future represents a value that will become available in the future.
//
// This is loosely based on this paper: http://www.home.hs-karlsruhe.de/~suma0002/publications/events-to-futures.pdf
type Future interface {
	AndThen(func(context.Context, Value) (Value, context.Context, error)) Future
	// TODO: add OrElse implementation
	// OrElse(func(context.Context, error) (Value, context.Context, error)) Future
	Get() (Value, context.Context, error)
	Cancel()
}

// Func wraps a nullary function with a context handling function
func Func(fn func() (Value, error)) func(context.Context) (Value, context.Context, error) {
	return func(ctx context.Context) (Value, context.Context, error) {
		v, e := fn()
		return v, ctx, e
	}
}

// ThenFunc wraps a continuation function with a context handling function
func ThenFunc(fn func(Value) (Value, error)) func(context.Context, Value) (Value, context.Context, error) {
	return func(ctx context.Context, vo Value) (Value, context.Context, error) {
		v, e := fn(vo)
		return v, ctx, e
	}
}

// Do creates a future that executes the function in a go routine
// The context that is passed into the function will provide a cancellation signal
func Do(fn func(context.Context) (Value, context.Context, error)) Future {
	return DoWithContext(context.Background(), fn)
}

// DoWithContext creates a future that executes the function in a go routine.
// The context is passed into the function so that it can be used for handling cancellation
// The function is expected to either pass the original context along or provide a new context based off the one passed in.
func DoWithContext(ctx context.Context, fn func(context.Context) (Value, context.Context, error)) Future {
	inner, cancel := context.WithCancel(ctx)
	c := make(chan result, 1)
	go func() {
		defer close(c)
		v, ctx, e := fn(inner)
		c <- result{Value: v, Err: e, Ctx: ctx}
	}()

	return &future{
		C:      c,
		cancel: cancel,
		ctx:    inner,
		once:   new(sync.Once),
	}
}

type future struct {
	C      chan result
	cancel context.CancelFunc
	ctx    context.Context
	once   *sync.Once
	val    *result
}

func (f *future) Get() (Value, context.Context, error) {
	f.once.Do(func() {
		select {
		case <-f.ctx.Done():
			f.val = &result{Err: f.ctx.Err(), Ctx: f.ctx}
		case val := <-f.C:
			f.val = &val
		}
	})
	if f.val == nil {
		return nil, f.ctx, nil
	}
	return f.val.Value, f.val.Ctx, f.val.Err
}

func (f *future) Cancel() {
	f.cancel()
}

func (f *future) AndThen(fn func(context.Context, Value) (Value, context.Context, error)) Future {
	c := make(chan result, 1)
	go func() {
		defer close(c)

		v, ctx, e := f.Get()
		if e != nil { // on error we fail here
			c <- result{Value: v, Ctx: ctx, Err: e}
			return
		}

		select {
		case <-ctx.Done():
			c <- result{Value: v, Ctx: ctx, Err: ctx.Err()}
			select {
			case <-f.ctx.Done():
			default:
				f.cancel() // ensure closed if out of scope
			}
		case <-f.ctx.Done():
			c <- result{Value: v, Ctx: ctx, Err: f.ctx.Err()}
		default:
			vv, ctx2, e2 := fn(ctx, v)
			c <- result{Value: vv, Ctx: ctx2, Err: e2}
		}

	}()
	return &future{
		C:      c,
		cancel: f.cancel,
		ctx:    f.ctx,
		once:   new(sync.Once),
	}
}
