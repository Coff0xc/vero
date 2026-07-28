package llm

import "github.com/Coff0xc/vero/internal/core"

// targetInjector —— 包装任意决策器: LLM 未在 args 填 target(或填空)时, 自动注入战役目标。
// 让 LLM 专注"选哪个工具/为什么", 目标主机由编排层保证 —— 减少 LLM 出错面。
type targetInjector struct {
	inner  core.LLM
	target string
}

// WithTarget —— 给决策器套上 target 注入。透传 Rejecter 能力(动态重规划)。
func WithTarget(inner core.LLM, target string) core.LLM {
	return &targetInjector{inner: inner, target: target}
}

func (t *targetInjector) Propose(goal string, g *core.AttackGraph, h []core.HistoryItem) *core.Action {
	a := t.inner.Propose(goal, g, h)
	if a != nil {
		if a.Args == nil {
			a.Args = map[string]any{}
		}
		if v, ok := a.Args["target"].(string); !ok || v == "" {
			a.Args["target"] = t.target
		}
	}
	return a
}

func (t *targetInjector) OnReject() {
	if r, ok := t.inner.(core.Rejecter); ok {
		r.OnReject()
	}
}
