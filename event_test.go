package MyGoBus

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	bus := New()
	if bus == nil {
		t.Log("New EventBus not created!")
		t.Fail()
	}
}

func TestSubscribe(t *testing.T) {
	bus := New()
	if bus.Subscribe("topic", func() {}) != nil {
		t.Fail()
	}
	if bus.Subscribe("topic", "String") == nil {
		t.Fail()
	}
}

func TestHasCallBack(t *testing.T) {
	bus := New()
	bus.Subscribe("topic", func() {})
	if bus.HasCallBack("topic_topic") {
		t.Fail()
	}
	if !bus.HasCallBack("topic") {
		t.Fail()
	}
}

func TestSubscribeOnce(t *testing.T) {
	bus := New()
	if bus.SubscribeOnce("topic", func() {}) != nil {
		t.Fail()
	}
	if bus.SubscribeOnce("topic", "String") == nil {
		t.Fail()
	}
}

func TestSubscribeOnceAndManySubscribe(t *testing.T) {
	bus := New()
	event := "topic"

	flag := 0
	fn := func() { flag += 1 }
	// 3 subscibe
	bus.SubscribeOnce(event, fn)
	bus.Subscribe(event, fn)
	bus.Subscribe(event, fn)

	bus.Publish(event)
	t.Logf("flag: %d", flag)

	if flag != 3 {
		t.Fail()
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := New()
	handler := func() {}

	bus.Subscribe("topic", handler)
	if bus.UnSubscribe("topic", handler) != nil {
		t.Fail()
	}
	if bus.UnSubscribe("topic", handler) == nil {
		t.Fail()
	}
}

type handler struct {
	val int
}

func (h *handler) Handle() {
	h.val++
}

func TestUnsubscribeMethod(t *testing.T) {
	bus := New()
	h := &handler{val: 0}

	bus.Subscribe("topic", h.Handle)
	bus.Publish("topic")
	if bus.UnSubscribe("topic", h.Handle) != nil {
		t.Fail()
	}
	if bus.UnSubscribe("topic", h.Handle) == nil {
		t.Fail()
	}
	bus.Publish("topic")
	bus.WaitAsync()

	if h.val != 1 {
		t.Fail()
	}
}

func TestPublish(t *testing.T) {
	bus := New()
	bus.Subscribe("topic", func(a int, err error) {
		if a != 10 {
			t.Fail()
		}

		if err != nil {
			t.Fail()
		}
	})
	bus.Publish("topic", 10, nil)
}

func TestSubcribeOnceAsync(t *testing.T) {
	results := make([]int, 0)

	bus := New()
	bus.SubscribeOnceAsync("topic", func(a int, out *[]int) {
		*out = append(*out, a)
	})

	bus.Publish("topic", 10, &results)
	bus.Publish("topic", 10, &results)
	bus.WaitAsync()

	t.Logf("len(results) == %d", len(results))

	if len(results) != 1 {
		t.Fail()
	}

	if bus.HasCallBack("topic") {
		t.Fail()
	}
}

func TestSubscribeAsyncTransactional(t *testing.T) {
	results := make([]int, 0)

	bus := New()
	bus.SubscribeAsync("topic", func(a int, out *[]int, duration string) {
		sleep, _ := time.ParseDuration(duration)
		time.Sleep(sleep)
		*out = append(*out, a)
	}, true)

	bus.Publish("topic", 1, &results, "1s")
	bus.Publish("topic", 11, &results, "2s")
	bus.WaitAsync()

	if len(results) != 2 {
		t.Fail()
	}

	if results[0] != 1 || results[1] != 11 {
		t.Fail()
	}
}

func TestSubscribeAsync(t *testing.T) {
	results := make(chan int, 2) // 增加缓冲以避免阻塞

	bus := New()
	var numResults int32

	bus.SubscribeAsync("topic", func(a int, out chan<- int) {
		out <- a
	}, false)

	bus.Publish("topic", 1, results)
	bus.Publish("topic", 2, results)

	go func() {
		for range results {
			atomic.AddInt32(&numResults, 1)
		}
	}()

	bus.WaitAsync()
	close(results) // 确保关闭通道以结束循环

	time.Sleep(10 * time.Millisecond)

	// 使用原子读取操作来获取numResults的值
	finalResults := atomic.LoadInt32(&numResults)
	if finalResults != 2 {
		t.Errorf("Expected 2 results, got %d", finalResults)
	}
}

func TestPubSub_Unsubscribe(t *testing.T) {
	capacity, testIndex := 100, 76
	bus := New()
	topicName := "TestPubSub_UnSubscribe"

	chs := make([]chan bool, capacity)
	for i := 0; i < capacity; i++ {
		chs[i] = make(chan bool, 1) // 初始化每个通道，缓冲大小为1
	}

	// 创建多个函数，每个函数内部都有相同的指针引用
	funcs := make([]func(event bool), capacity) // 创建一个函数切片，用于存储订阅的函数
	for i := 0; i < capacity; i++ {
		i := i                  // 创建局部变量i，避免闭包中使用外部循环变量i
		f := func(event bool) { // 定义一个匿名函数，接收一个布尔类型的事件
			chs[i] <- event // 将事件发送到对应的通道中
		}
		funcs[i] = f
		bus.Subscribe(topicName, f)
	}

	// 发布消息，并检查是否收到信号
	bus.Publish(topicName, true)
	for i := 0; i < capacity; i++ {
		select {
		case <-chs[i]: // 尝试从通道中接收消息
			// 信号接收成功
		case <-time.After(time.Millisecond): // 如果在指定时间内没有接收到消息，则认为测试失败
			t.Errorf("(%d) signal should be received before Unsubscribe", i)
		}
	}

	bus.UnSubscribe(topicName, funcs[testIndex]) // 取消订阅，传入主题名称和要取消订阅的函数

	// 再次发布消息，并检查是否未收到信号
	bus.Publish(topicName, true)
	select {
	case <-chs[testIndex]: // 尝试从被取消订阅的通道中接收消息
		t.Error("signal should not be received") // 如果接收到消息，则测试失败
	case <-time.After(time.Millisecond): // 如果在指定时间内没有接收到消息，则认为成功
		// 信号未接收
	}

	// 检查其他所有信号是否接收
	for i := 0; i < capacity-1; i++ {
		select {
		case <-chs[i]: // 尝试从通道中接收消息
			// 信号接收成功
		case <-time.After(time.Millisecond): // 如果在指定时间内没有接收到消息
			if i == testIndex {
				continue // 如果是已取消订阅的索引，则跳过
			}
			t.Errorf("(%d) signal should be received after Unsubscribe", i)
		}
	}
}
