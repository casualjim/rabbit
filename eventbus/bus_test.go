package eventbus

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegisterHandlers(t *testing.T) {
	bus := New(nil)
	defer bus.Close()
	assert.Equal(t, 0, bus.Len())
	bus.Subscribe(NOOPHandler)
	assert.Equal(t, 1, bus.Len())
}

func TestUnregisterHandlers(t *testing.T) {
	bus := New(nil)
	defer bus.Close()

	assert.Equal(t, 0, bus.Len())
	bus.Subscribe(NOOPHandler, NOOPHandler, NOOPHandler)
	assert.Equal(t, 3, bus.Len())
	bus.Unsubscribe(NOOPHandler)
	assert.Equal(t, 2, bus.Len())
}

func TestPublish_ToAllListeners(t *testing.T) {
	bus := New(nil)
	defer bus.Close()

	evts := make([]Event, 3)
	wg := new(sync.WaitGroup)
	wg.Add(3)
	lock := new(sync.Mutex)
	var seen int
	listener1 := Handler(func(evt Event) error {
		lock.Lock()
		evts[0] = evt
		seen++
		lock.Unlock()

		wg.Done()
		return nil
	})
	listener2 := Handler(func(evt Event) error {
		lock.Lock()
		evts[1] = evt
		seen++
		lock.Unlock()

		wg.Done()
		return nil
	})
	listener3 := Handler(func(evt Event) error {
		lock.Lock()
		evts[2] = evt
		seen++
		lock.Unlock()

		wg.Done()
		return nil
	})

	bus.Subscribe(listener1, listener2, listener3)

	evt := Event{Name: "the event"}
	bus.Publish(evt)
	wg.Wait()
	assert.EqualValues(t, evt, evts[0])
	assert.EqualValues(t, evt, evts[1])
	assert.EqualValues(t, evt, evts[2])
}

func TestSubscribeFilter(t *testing.T) {
	latch := make(chan struct{})
	var count int64
	handler := Handler(func(evt Event) error {
		atomic.AddInt64(&count, 1)
		if event, ok := evt.Args.(string); ok && event == "correct-id" {
			latch <- struct{}{}
		}
		return nil
	})

	var filterCount int64
	matchTriggered := func(evt Event) bool {
		atomic.AddInt64(&filterCount, 1)
		return evt.Name == "trigger"
	}

	filtered := Filtered(matchTriggered, handler)

	bus := New(nil)
	defer bus.Close()

	bus.Subscribe(filtered)
	bus.Publish(newTestEvent("trigger", "wrong-id"))
	noMessageWithin(t, 300*time.Millisecond, latch)

	bus.Publish(newTestEvent("no-trigger", "correct-id"))
	noMessageWithin(t, 300*time.Millisecond, latch)

	bus.Publish(newTestEvent("trigger", "correct-id"))
	messageWithin(t, 300*time.Millisecond, latch)
	assert.EqualValues(t, 2, count)
	assert.EqualValues(t, 3, filterCount)
}

func noMessageWithin(t testing.TB, dur time.Duration, ch chan struct{}) {
	select {
	case msg := <-ch:
		assert.Fail(t, "expected no message", "got %+v", msg)
	case <-time.After(dur):
	}
}

func messageWithin(t testing.TB, dur time.Duration, ch chan struct{}) {
	select {
	case <-ch:
	case <-time.After(dur):
		assert.Fail(t, "expected to have received a message", "timeout after %v", dur)
	}
}

func newTestEvent(name, data string) Event {
	return Event{
		Name: name,
		At:   time.Now(),
		Args: data,
	}
}
