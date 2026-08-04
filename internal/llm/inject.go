package llm

import (
	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/tools"
)

// targetInjector —— 包装任意决策器: LLM 未在 args 填 target(或填空)时, 自动注入战役目标。
// 让 LLM 专注"选哪个工具/为什么", 目标主机由编排层保证 —— 减少 LLM 出错面。
//
// 能力透传: 包装器必须透传被包装决策器的全部可选能力(Planner/ErrorReporter/
// ThinkingReporter/Observer/BattleReflector/Reflector/Rejecter), 否则内核的类型断言
// 命中的是包装器 —— 曾经全部断言失败, 导致: 多步规划/深度思考/LLM-as-observer/战役反思
// 静默失效, 且 API 失败原因(401/无效模型名)被吞, 战役空转只报 "no more actions"。
type targetInjector struct {
	inner  core.LLM
	target string
}

// WithTarget —— 给决策器套上 target 注入。透传全部可选能力(见 targetInjector 注释)。
func WithTarget(inner core.LLM, target string) core.LLM {
	return &targetInjector{inner: inner, target: target}
}

// injectTarget —— 动作缺 target 时注入战役目标。
func (t *targetInjector) injectTarget(a *core.Action) {
	if a.Args == nil {
		a.Args = map[string]any{}
	}
	if v, ok := a.Args["target"].(string); !ok || v == "" {
		a.Args["target"] = t.target
	}
}

func (t *targetInjector) Propose(goal string, g *core.AttackGraph, h []core.HistoryItem) *core.Action {
	a := t.inner.Propose(goal, g, h)
	if a != nil {
		t.injectTarget(a)
	}
	return a
}

// ProposePlan —— 透传 Planner(多步规划)并给计划里每个动作注入 target;
// 内层不支持多步规划时(如脚本 Mock)回退单步 Propose 包成一步计划, 行为不变。
func (t *targetInjector) ProposePlan(goal string, g *core.AttackGraph, h []core.HistoryItem) *core.Plan {
	if pl, ok := t.inner.(core.Planner); ok {
		p := pl.ProposePlan(goal, g, h)
		if p != nil {
			for i := range p.Actions {
				t.injectTarget(&p.Actions[i])
			}
		}
		return p
	}
	a := t.Propose(goal, g, h)
	if a == nil {
		return nil
	}
	return &core.Plan{Actions: []core.Action{*a}}
}

// LastError —— 透传 ErrorReporter: 内核 "no more actions" 时把真实失败原因
// (API 401/无效模型名/网络故障)广播给前端, 不再静默空转。
func (t *targetInjector) LastError() string {
	if er, ok := t.inner.(core.ErrorReporter); ok {
		return er.LastError()
	}
	return ""
}

// LastThinking —— 透传 ThinkingReporter(深度思考思维链, 前端折叠展示)。
func (t *targetInjector) LastThinking() string {
	if tr, ok := t.inner.(core.ThinkingReporter); ok {
		return tr.LastThinking()
	}
	return ""
}

// Observe —— 透传 Observer(LLM-as-parser: 固定 parser 无产出时的兜底理解)。
func (t *targetInjector) Observe(tool string, args map[string]any, stdout string) []tools.Observation {
	if ob, ok := t.inner.(core.Observer); ok {
		return ob.Observe(tool, args, stdout)
	}
	return nil
}

// Reflect —— 透传 BattleReflector(战役级战略反思, 每 N 步注入下轮 prompt)。
func (t *targetInjector) Reflect(goal string, g *core.AttackGraph, h []core.HistoryItem) string {
	if br, ok := t.inner.(core.BattleReflector); ok {
		return br.Reflect(goal, g, h)
	}
	return ""
}

func (t *targetInjector) OnReject() {
	if r, ok := t.inner.(core.Rejecter); ok {
		r.OnReject()
	}
}

// OnFailure —— 透传 Reflector 能力: 内核在失败/被拒时回传的动作+原因, 原样交给被包装的决策器。
func (t *targetInjector) OnFailure(action core.Action, reason string) {
	if rf, ok := t.inner.(core.Reflector); ok {
		rf.OnFailure(action, reason)
	}
}
