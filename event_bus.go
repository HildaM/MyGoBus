package MyGoBus

import (
	"fmt"
	"reflect"
	"sync"
)

// BusSubscriber subscription bus interface
type BusSubscriber interface {
	Subscribe(topic string, fn interface{}) error
	SubscribeAsync(topic string, fn interface{}, transactional bool) error
	SubscribeOnce(topic string, fn interface{}) error
	SubscribeOnceAsync(topic string, fn interface{}) error
	UnSubscribe(topic string, handler interface{}) error
}

// BusPublisher publish bus interface
type BusPublisher interface {
	Publish(topic string, args ...interface{})
}

// BusController bus control interface
type BusController interface {
	HasCallBack(topic string) bool
	WaitAsync()
}

// Bus global bus behavior
type Bus interface {
	BusController
	BusSubscriber
	BusPublisher
}

// EventBus structure for handlers and callbacks
type EventBus struct {
	handlers  map[string][]*eventHandler
	lock      sync.Mutex // lock for map
	asyncLock sync.Mutex // Use for WaitAsync mode only
	wg        sync.WaitGroup
}

// eventHandler single event handler
type eventHandler struct {
	callBack      reflect.Value
	once          *sync.Once
	async         bool
	transactional bool
	sync.Mutex    // lock for event handler, useful for running async callbacks
}

// New returns new EventBus with empty handlers.
func New() Bus {
	b := &EventBus{
		handlers:  make(map[string][]*eventHandler),
		lock:      sync.Mutex{},
		asyncLock: sync.Mutex{},
		wg:        sync.WaitGroup{},
	}
	return Bus(b)
}

// doSubscribe handles the subscription logic and is utilized by the public Subscribe functions
func (bus *EventBus) doSubscribe(topic string, fn interface{}, handler *eventHandler) error {
	bus.lock.Lock()
	defer bus.lock.Unlock()

	if !(reflect.TypeOf(fn).Kind() == reflect.Func) {
		return fmt.Errorf("%s is not of type reflect.Func", reflect.TypeOf(fn).Kind())
	}
	bus.handlers[topic] = append(bus.handlers[topic], handler)
	return nil
}

// Subscribe subscribes to a topic.
// Returns error if `fn` is not a function.
func (bus *EventBus) Subscribe(topic string, fn interface{}) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), nil, false, false, sync.Mutex{},
	})
}

// SubscribeAsync subscribes to a topic with an asynchronous callback
// Transactional determines whether subsequent callbacks for a topic are
// run serially (true) or concurrently (false)
// Returns error if `fn` is not a function.
func (bus *EventBus) SubscribeAsync(topic string, fn interface{}, transactional bool) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), nil, true, transactional, sync.Mutex{},
	})
}

// SubscribeOnce subscribes to a topic once. Handler will be removed after executing.
// Returns error if `fn` is not a function.
func (bus *EventBus) SubscribeOnce(topic string, fn interface{}) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), new(sync.Once), false, false, sync.Mutex{},
	})
}

// SubscribeOnceAsync subscribes to a topic once with an asynchronous callback
// Handler will be removed after executing.
// Returns error if `fn` is not a function.
func (bus *EventBus) SubscribeOnceAsync(topic string, fn interface{}) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), new(sync.Once), true, false, sync.Mutex{},
	})
}

func (bus *EventBus) UnSubscribe(topic string, handler interface{}) error {
	bus.lock.Lock()
	defer bus.lock.Unlock()

	if _, ok := bus.handlers[topic]; ok && len(bus.handlers[topic]) > 0 {
		bus.removeHandler(topic, bus.findHandlerIndex(topic, reflect.ValueOf(handler)))
		return nil
	}
	return fmt.Errorf("topic %s doesn't exist", topic)
}

func (bus *EventBus) removeHandler(topic string, idx int) {
	if _, ok := bus.handlers[topic]; !ok {
		return
	}

	l := len(bus.handlers[topic])

	if !(0 <= idx && idx < l) {
		return
	}

	// remove handlers
	copy(bus.handlers[topic][idx:], bus.handlers[topic][idx+1:])
	bus.handlers[topic][l-1] = nil
	bus.handlers[topic] = bus.handlers[topic][:l-1]
}

func (bus *EventBus) findHandlerIndex(topic string, callBack reflect.Value) int {
	if _, ok := bus.handlers[topic]; ok {
		for idx, handler := range bus.handlers[topic] {
			// if handler.callBack.Type() == callBack.Type() &&
			// 	handler.callBack.Pointer() == callBack.Pointer() &&
			// 	reflect.DeepEqual(handler.callBack, callBack) {
			// 	// Creating functions within a loop in Go can result in unexpected behavior, such as multiple functions sharing the same pointer value.
			// 	// The reflect.DeepEqual(handler.callBack, callback) check has been includedas a final measure to distinguish between different values,
			// 	// particularly when the pointers and types are identical.
			// 	return idx
			// }

			// TODO bug in TestPubSub_Unsubscribe()
			if handler.callBack.Type() == callBack.Type() &&
				handler.callBack.Pointer() == callBack.Pointer() {
				return idx
			}
		}
	}
	return -1
}

// Publish executes callback defined for a topic. Any additional argument will be transferred to the callback.
func (bus *EventBus) Publish(topic string, args ...interface{}) {
	// Handlers slice may be changed by removeHandler and Unsubscribe during iteration,
	// so make a copy and iterate the copied slice.
	bus.lock.Lock()
	handlers := bus.handlers[topic]
	copyHanlders := make([]*eventHandler, len(handlers))
	copy(copyHanlders, handlers)
	bus.lock.Unlock()

	for _, handler := range copyHanlders {
		if !handler.async {
			bus.doPublish(handler, topic, args...)
		} else {
			bus.wg.Add(1)

			if handler.transactional {
				handler.Lock()
			}
			go bus.doPublishAsync(handler, topic, args...)
		}
	}
}

func (bus *EventBus) doPublishAsync(handler *eventHandler, topic string, args ...interface{}) {
	defer bus.wg.Done()
	if handler.transactional {
		defer handler.Unlock()
	}
	bus.doPublish(handler, topic, args...)
}

func (bus *EventBus) doPublish(handler *eventHandler, topic string, args ...interface{}) {
	passedArguments := bus.setUpPublish(handler, args...)

	if handler.once == nil {
		handler.callBack.Call(passedArguments)
		return
	}

	handler.once.Do(func() {
		// remove handler already processed
		bus.lock.Lock()
		for idx, h := range bus.handlers[topic] {
			// compare pointers since pointers are unique for all members of slice
			if h.once == handler.once {
				bus.removeHandler(topic, idx)
				break
			}
		}
		bus.lock.Unlock()

		handler.callBack.Call(passedArguments)
	})
}

func (bus *EventBus) setUpPublish(callback *eventHandler, args ...interface{}) []reflect.Value {
	funcType := callback.callBack.Type()
	passedArguments := make([]reflect.Value, len(args))
	for i, v := range args {
		if v == nil {
			passedArguments[i] = reflect.New(funcType.In(i)).Elem() // It returns the zero Value if v is nil.
		} else {
			passedArguments[i] = reflect.ValueOf(v)
		}
	}
	return passedArguments
}

// HasCallback returns true if exists any callback subscribed to the topic.
func (bus *EventBus) HasCallBack(topic string) bool {
	bus.lock.Lock()
	defer bus.lock.Unlock()

	if _, ok := bus.handlers[topic]; ok {
		return len(bus.handlers[topic]) > 0
	}
	return false
}

// WaitAsync waits for all async callbacks to complete
func (bus *EventBus) WaitAsync() {
	bus.wg.Wait()
}
