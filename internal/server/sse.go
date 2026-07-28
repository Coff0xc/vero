package server

import (
	"sync"

	"redcell/internal/core"
)

// Broker —— SSE 广播中枢: 每个连接一个 channel, Emit 广播给所有订阅者。
// 断开即注销 -> 浏览器重连/多标签健壮(对应 Python server.py 的 subscribers 队列列表)。
type Broker struct {
	mu   sync.Mutex
	subs map[chan core.Event]struct{}
}

func NewBroker() *Broker {
	return &Broker{subs: map[chan core.Event]struct{}{}}
}

// Subscribe —— 新连接订阅, 返回其 channel 与注销函数。
func (b *Broker) Subscribe() (chan core.Event, func()) {
	ch := make(chan core.Event, 64) // 带缓冲: 慢消费者不阻塞广播
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, ch)
			close(ch)
			b.mu.Unlock()
		})
	}
}

// Emit —— 广播一个事件。订阅者缓冲满就丢弃该事件, 保证战役不被慢客户端拖死。
func (b *Broker) Emit(e core.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
