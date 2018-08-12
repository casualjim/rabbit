package eventbus

import (
	"sync"
	"time"

	"github.com/casualjim/rabbit"
	"github.com/rcrowley/go-metrics"
)

// Event you can subscribe to
type Event struct {
	Name string
	At   time.Time
	Args interface{}
}

// NOOPHandler drops events on the floor without taking action
var NOOPHandler = Handler(func(_ Event) error { return nil })

// Handler wraps a function that will be called when an event is received
// In this mode the handler is quiet when an error is produced by the handler
// so the user of the eventbus needs to handle that error
func Handler(on func(Event) error) EventHandler {
	return &defaultHandler{
		on: on,
	}
}

// Forward is an event handler that forwards events to another event bus
func Forward(bus EventBus) EventHandler {
	return Handler(func(evt Event) error {
		bus.Publish(evt)
		return nil
	})
}

type defaultHandler struct {
	on func(Event) error
}

// On event trigger
func (h *defaultHandler) On(event Event) error {
	return h.on(event)
}

func newSubscription(handler EventHandler, errorHandler func(error)) *eventSubcription {
	return &eventSubcription{
		handler: handler,
		onError: errorHandler,
	}
}

type eventSubcription struct {
	listener chan Event
	handler  EventHandler
	once     sync.Once
	onError  func(error)
	lck      sync.Mutex
}

func (e *eventSubcription) Listen() {
	e.once.Do(func() {
		e.listener = make(chan Event)
		go func() {
			e.lck.Lock()
			for evt := range e.listener {
				if err := e.handler.On(evt); err != nil {
					e.onError(err)
				}
			}
			e.lck.Unlock()
		}()
	})
}

func (e *eventSubcription) Stop() {
	close(e.listener)
	e.lck.Lock()
	e.listener = nil
	e.once = sync.Once{}
	e.lck.Unlock()
}

func (e *eventSubcription) Matches(handler EventHandler) bool {
	return e.handler == handler
}

// EventHandler deals with handling events
type EventHandler interface {
	On(Event) error
}

type filteredHandler struct {
	Next    EventHandler
	Matches EventPredicate
}

func (f *filteredHandler) On(evt Event) error {
	if !f.Matches(evt) {
		return nil
	}
	return f.Next.On(evt)
}

// EventPredicate for filtering events
type EventPredicate func(Event) bool

// Filtered composes an event handler with a filter
func Filtered(matches EventPredicate, next EventHandler) EventHandler {
	return &filteredHandler{
		Matches: matches,
		Next:    next,
	}
}

// NopBus represents a zero value for an event bus
var NopBus EventBus = &nopBus{}

type nopBus struct {
}

func (b *nopBus) Close() error                { return nil }
func (b *nopBus) Publish(Event)               {}
func (b *nopBus) Subscribe(...EventHandler)   {}
func (b *nopBus) Unsubscribe(...EventHandler) {}
func (b *nopBus) Len() int                    { return 0 }

// EventBus does fanout to registered channels
type EventBus interface {
	Close() error
	Publish(Event)
	Subscribe(...EventHandler)
	Unsubscribe(...EventHandler)
	Len() int
}

type defaultEventBus struct {
	lock *sync.RWMutex

	channel      chan Event
	handlers     []*eventSubcription
	closing      chan chan struct{}
	log          rabbit.Logger
	errorHandler func(error)
}

// New event bus with specified logger
func New(log rabbit.Logger) EventBus {
	return NewWithTimeout(log, 100*time.Millisecond)
}

// NewWithTimeout creates a new eventbus with a timeout after which an event handler gets canceled
func NewWithTimeout(log rabbit.Logger, timeout time.Duration) EventBus {
	if log == nil {
		log = rabbit.NopLogger
	}
	e := &defaultEventBus{
		closing:      make(chan chan struct{}),
		channel:      make(chan Event, 100),
		log:          log,
		lock:         new(sync.RWMutex),
		errorHandler: func(err error) { log.Errorf(err.Error()) },
	}
	go e.dispatcherLoop(timeout)
	return e
}

func (e *defaultEventBus) dispatcherLoop(timeout time.Duration) {
	totWait := new(sync.WaitGroup)
	for {
		select {
		case evt := <-e.channel:
			e.log.Infof("Got event %+v in channel\n", evt)
			timer := metrics.GetOrRegisterTimer("events.notify", metrics.DefaultRegistry)
			go timer.Time(func() {
				totWait.Add(1)
				e.lock.RLock()

				noh := len(e.handlers)
				if noh == 0 {
					e.log.Infof("there are no active listeners, skipping broadcast")
					e.lock.RUnlock()
					totWait.Done()
					return
				}

				var wg sync.WaitGroup
				wg.Add(noh)
				e.log.Infof("notifying %d listeners", noh)
				for _, handler := range e.handlers {
					go func(listener chan<- Event) {
						timer := time.NewTimer(timeout)
						select {
						case listener <- evt:
							timer.Stop()
						case <-timer.C:
							e.log.Infof("Failed to send event %+v to listener within %v", evt, timeout)
						}
						wg.Done()
					}(handler.listener)
				}

				wg.Wait()
				e.lock.RUnlock()
				totWait.Done()
			})
		case closed := <-e.closing:
			totWait.Wait()
			close(e.channel)
			e.lock.Lock()
			for _, h := range e.handlers {
				h.Stop()
			}
			e.handlers = nil
			e.lock.Unlock()

			closed <- struct{}{}
			close(closed)
			e.log.Infof("event bus closed")
			return
		}
	}
}

// SetErrorHandler changes the default error handler which logs as error
// to the new error handler provided to this method
func (e *defaultEventBus) SetErrorHandler(handler func(error)) {
	e.lock.Lock()
	e.errorHandler = handler
	e.lock.Unlock()
}

// Publish an event to all interested subscribers
func (e *defaultEventBus) Publish(evt Event) {
	e.channel <- evt
}

// Subscribe to events published in the bus
func (e *defaultEventBus) Subscribe(handlers ...EventHandler) {
	e.lock.Lock()
	e.log.Infof("adding %d listeners", len(handlers))
	for _, handler := range handlers {
		sub := newSubscription(handler, e.errorHandler)
		e.handlers = append(e.handlers, sub)
		sub.Listen()
	}
	e.lock.Unlock()
}

func (e *defaultEventBus) Unsubscribe(handlers ...EventHandler) {
	e.lock.Lock()
	if len(e.handlers) == 0 {
		e.log.Infof("nothing to remove from %d handlers", len(handlers))
		e.lock.Unlock()
		return
	}
	e.log.Infof("removing %d listeners", len(handlers))
	for _, h := range handlers {
		for i, handler := range e.handlers {
			if handler.Matches(h) {
				handler.Stop()
				// replace handler because it will still process messages in flight
				e.handlers = append(e.handlers[:i], e.handlers[i+1:]...)
				break
			}
		}
	}
	e.lock.Unlock()
}

func (e *defaultEventBus) Close() error {
	e.log.Infof("closing eventbus")
	ch := make(chan struct{})
	e.closing <- ch
	<-ch
	close(e.closing)

	return nil
}

func (e *defaultEventBus) Len() int {
	e.log.Infof("getting the length of the handlers")
	e.lock.RLock()
	sz := len(e.handlers)
	e.lock.RUnlock()
	return sz
}
