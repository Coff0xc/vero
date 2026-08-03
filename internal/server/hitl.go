package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Coff0xc/vero/internal/core"
)

// HITLTimeout —— 操作员超时未裁决则默认拒绝(对应 Python 300s 超时)。
const HITLTimeout = 300 * time.Second

// gate —— 一次待审批: ch 等操作员裁决。
type gate struct {
	ch chan bool
}

// WebGate —— Web HITL: Approve 发 hitl_request 事件并阻塞等操作员裁决。
// 实现 core.Approve 签名, 直接作为 RunAgent 的审批钩子。
type WebGate struct {
	mu      sync.Mutex
	pending map[string]*gate
	broker  *Broker
	seq     int
}

func NewWebGate(b *Broker) *WebGate {
	return &WebGate{pending: map[string]*gate{}, broker: b}
}

// Approve —— core.Approve 实现: 发 hitl_request, 阻塞等操作员(超时默认拒绝)。
func (w *WebGate) Approve(a core.Action, level int) bool {
	return w.ApproveCtx(context.Background(), a, level)
}

// ApproveCtx —— 带取消的审批: 战役被取消时立即解除阻塞, 避免 HITL 等待挂死战役收尾。
func (w *WebGate) ApproveCtx(ctx context.Context, a core.Action, level int) bool {
	w.mu.Lock()
	w.seq++
	key := fmt.Sprintf("hitl-%d", w.seq) // 自增 key: 确定性、无碰撞(不用时间戳)
	g := &gate{ch: make(chan bool, 1)}
	w.pending[key] = g
	w.mu.Unlock()

	w.broker.Emit(core.Event{Kind: "hitl_request", Data: map[string]any{
		"key": key, "tool": a.Tool, "args": a.Args, "level": level, "why": a.Rationale,
	}})

	select {
	case ok := <-g.ch:
		return ok
	case <-time.After(HITLTimeout):
		w.mu.Lock()
		delete(w.pending, key)
		w.mu.Unlock()
		return false
	case <-ctx.Done(): // 取消战役: 默认拒绝并解除等待
		w.mu.Lock()
		delete(w.pending, key)
		w.mu.Unlock()
		return false
	}
}

// Resolve —— 操作员裁决(Approve handler 调用)。
func (w *WebGate) Resolve(key string, approved bool) {
	w.mu.Lock()
	g, ok := w.pending[key]
	if ok {
		delete(w.pending, key)
	}
	w.mu.Unlock()
	if ok {
		g.ch <- approved
	}
}

// CancelAll —— 新战役开始前放弃所有旧待审批(防僵尸线程)。
func (w *WebGate) CancelAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for key, g := range w.pending {
		select {
		case g.ch <- false:
		default:
		}
		delete(w.pending, key)
	}
}
