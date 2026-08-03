package core

import "github.com/Coff0xc/vero/internal/tools"

// LLM —— 决策接口: 看目标 + 攻击图, 提议下一步 Action(返回 nil 表示结束)。
// MockLLM / ClaudeLLM / PlannerLLM 都实现它 —— 内核不关心决策来自真实模型还是确定性规划器。
type LLM interface {
	Propose(goal string, g *AttackGraph, history []HistoryItem) *Action
}

// Rejecter —— 可选能力: 上一步被拒/失败时通知决策器换路(对应 Python on_reject)。
// PlannerLLM 实现它做动态重规划; MockLLM 不实现, 内核用类型断言探测。
type Rejecter interface {
	OnReject()
}

// Plan —— 一次提议产出的多步计划: 按序执行, 某步失败/被拒即中断后续步。
// 红队攻击链是依赖推进的(侦察→打点→凭证→横向), 一次给出整段计划让模型做全局推理,
// 而非每轮只挤下一步 —— 这是对"单步决策"架构缺陷(#44)的核心增强。
type Plan struct {
	Rationale string   // 计划级推理(为什么选这条链)
	Actions   []Action // 有序动作; 前序失败后续不执行
}

// Planner —— 可选能力: 一次提议给出多步计划。实现它的 LLM 走多步规划主循环;
// 只实现 Propose 的老式 LLM 仍按单步执行(向后兼容)。
type Planner interface {
	ProposePlan(goal string, g *AttackGraph, history []HistoryItem) *Plan
}

// HistoryItem —— 一条历史(动作 + 结果), 供决策器参考。
type HistoryItem struct {
	Outcome string // done / rejected
	Action  Action
	Result  *tools.ToolResult
}
