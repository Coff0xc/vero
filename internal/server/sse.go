package server

import (
	"fmt"
	"os"
	"sync"

	"github.com/Coff0xc/vero/internal/core"
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

// Emit —— 广播一个事件。常规事件缓冲满丢弃(保证战役不被慢客户端拖死);
// 审批类事件(hitl_request/hitl)缓冲满时记录告警 —— 不静默吞掉, 供排障;
// 补偿机制: 断线/丢失的审批可经 GET /api/approvals/pending 补缝, 不会永久丢。
func (b *Broker) Emit(e core.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	critical := e.Kind == "hitl_request" || e.Kind == "hitl"
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			if critical {
				fmt.Fprintf(os.Stderr, "[broker] 审批事件 %q 被慢消费者丢弃(缓冲满) — 前端可经 /api/approvals/pending 补缝\n", e.Kind)
			}
		}
	}
}
