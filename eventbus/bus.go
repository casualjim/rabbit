package eventbus

import (
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
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
		once:    new(sync.Once),
		onError: errorHandler,
	}
}

type eventSubcription struct {
	listener chan Event
	handler  EventHandler
	once     *sync.Once
	onError  func(error)
}

func (e *eventSubcription) Listen() {
	e.once.Do(func() {
		e.listener = make(chan Event)
		go func() {
			for evt := range e.listener {
				if err := e.handler.On(evt); err != nil {
					e.onError(err)
				}
			}
		}()
	})
}

func (e *eventSubcription) Stop() {
	close(e.listener)
	e.listener = nil
	e.once = new(sync.Once)
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
	log          logrus.FieldLogger
	errorHandler func(error)
}

// New event bus with specified logger
func New(log logrus.FieldLogger) EventBus {
	return NewWithTimeout(log, 100*time.Millisecond)
}

// NewWithTimeout creates a new eventbus with a timeout after which an event handler gets cancelled
func NewWithTimeout(log logrus.FieldLogger, timeout time.Duration) EventBus {
	if log == nil {
		log = logrus.New().WithFields(nil)
	}
	e := &defaultEventBus{
		closing:      make(chan chan struct{}),
		channel:      make(chan Event, 100),
		log:          log,
		lock:         new(sync.RWMutex),
		errorHandler: func(err error) { log.Errorln(err) },
	}
	go e.dispatcherLoop(timeout)
	return e
}

func (e *defaultEventBus) dispatcherLoop(timeout time.Duration) {
	totWait := new(sync.WaitGroup)
	for {
		select {
		case evt := <-e.channel:
			e.log.Debugf("Got event %+v in channel\n", evt)
			timer := metrics.GetOrRegisterTimer("events.notify", metrics.DefaultRegistry)
			go timer.Time(func() {
				totWait.Add(1)
				e.lock.RLock()

				noh := len(e.handlers)
				if noh == 0 {
					e.log.Debugf("there are no active listeners, skipping broadcast")
					e.lock.RUnlock()
					totWait.Done()
					return
				}

				var wg sync.WaitGroup
				wg.Add(noh)
				e.log.Debugf("notifying %d listeners", noh)
				for _, handler := range e.handlers {
					go func(listener chan<- Event) {
						timer := time.NewTimer(timeout)
						select {
						case listener <- evt:
							timer.Stop()
						case <-timer.C:
							e.log.Warnf("Failed to send event %+v to listener within %v", evt, timeout)
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
			e.log.Debug("event bus closed")
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
	e.log.Debugf("adding %d listeners", len(handlers))
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
		e.log.Debugf("nothing to remove from", len(handlers))
		e.lock.Unlock()
		return
	}
	e.log.Debugf("removing %d listeners", len(handlers))
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
	e.log.Debugf("closing eventbus")
	ch := make(chan struct{})
	e.closing <- ch
	<-ch
	close(e.closing)

	return nil
}

func (e *defaultEventBus) Len() int {
	e.log.Debugf("getting the length of the handlers")
	e.lock.RLock()
	sz := len(e.handlers)
	e.lock.RUnlock()
	return sz
}
