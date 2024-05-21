# MyGoBus --- Lightweight eventbus with async compatibility for Go

## What it is

Package EventBus is the little and lightweight eventbus with async compatibility for GoLang.

## Feature

Based on [EventBus](https://github.com/asaskevich/EventBus), but fixed many error on original repo.

## QuickStart
### Installation
	go get github.com/HildaM/MyGoBus

### Import package in your project

    import "github.com/HildaM/MyGoBus"

### Example
```go
func mygo(msg string) {
	fmt.Printf("MyGo Hello: %s\n", msg)
}

func main() {
	bus := EventBus.New();
	bus.Subscribe("main:mygo", mygo);
	bus.Publish("main:mygo", "Anno");
	bus.Unsubscribe("main:mygo", mygo);
}
```

## Implemented methods

* **New()**
* **Subscribe()**
* **SubscribeOnce()**
* **HasCallBack()**
* **UnSubscribe()**
* **Publish()**
* **SubscribeAsync()**
* **SubscribeOnceAsync()**
* **WaitAsync()**

### New()
New returns new EventBus with empty handlers.
```go
bus := EventBus.New();
```

### Subscribe(topic string, fn interface{}) error
Subscribe to a topic. Returns error if `fn` is not a function.
```go
func Handler() { ... }
...
bus.Subscribe("topic:handler", Handler)
```

### SubscribeOnce(topic string, fn interface{}) error
Subscribe to a topic once. Handler will be removed after executing. Returns error if `fn` is not a function.
```go
func HelloWorld() { ... }
...
bus.SubscribeOnce("topic:handler", HelloWorld)
```

### UnSubscribe(topic string, fn interface{}) error
Remove callback defined for a topic. Returns error if there are no callbacks subscribed to the topic.
```go
bus.UnSubscribe("topic:handler", HelloWord);
```

### HasCallBack(topic string) bool
Returns true if exists any callback subscribed to the topic.

### Publish(topic string, args ...interface{})
Publish executes callback defined for a topic. Any additional argument will be transferred to the callback.
```go
func Handler(str string) { ... }
...
bus.Subscribe("topic:handler", Handler)
...
bus.Publish("topic:handler", "Hello, World!");
```

### SubscribeAsync(topic string, fn interface{}, transactional bool)
Subscribe to a topic with an asynchronous callback. Returns error if `fn` is not a function.
```go
func slowCalculator(a, b int) {
	time.Sleep(3 * time.Second)
	fmt.Printf("%d\n", a + b)
}

bus := EventBus.New()
bus.SubscribeAsync("main:slow_calculator", slowCalculator, false)

bus.Publish("main:slow_calculator", 20, 60)

fmt.Println("start: do some stuff while waiting for a result")
fmt.Println("end: do some stuff while waiting for a result")

bus.WaitAsync() // wait for all async callbacks to complete

fmt.Println("do some stuff after waiting for result")
```
Transactional determines whether subsequent callbacks for a topic are run serially (true) or concurrently(false)

### SubscribeOnceAsync(topic string, args ...interface{})
SubscribeOnceAsync works like SubscribeOnce except the callback to executed asynchronously

###  WaitAsync()
WaitAsync waits for all async callbacks to complete.

### Cross Process Events
Works with two rpc services:
- a client service to listen to remotely published events from a server
- a server service to listen to client subscriptions

server.go
```go
func main() {
    server := NewServer(":2010", "/_server_bus_", New())
    server.Start()
    // ...
    server.EventBus().Publish("main:calculator", 4, 6)
    // ...
    server.Stop()
}
```

client.go
```go
func main() {
    client := NewClient(":2015", "/_client_bus_", New())
    client.Start()
    client.Subscribe("main:calculator", calculator, ":2010", "/_server_bus_")
    // ...
    client.Stop()
}
```