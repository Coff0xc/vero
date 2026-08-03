package core

import "github.com/Coff0xc/vero/internal/tools"

// LLM —— 决策接口: 看目标 + 攻击图, 提议下一步 Action(返回 nil 表示结束)。
// MockLLM / ClaudeLLM / PlannerLLM 都实现它 —— 内核不关心决策来自真实模型还是确定性规划器。
type LLM interface {
	Propose(goal string, g *AttackGraph, history []HistoryItem) *Action
}

// ErrorReporter —— 可选能力: 决策器最近一次决策失败的原因(如 API 错误/无效模型名),
// 供内核在 "no more actions" 时向前端广播 error 事件, 让用户看到为什么战役空转。
type ErrorReporter interface {
	LastError() string
}

// Rejecter —— 可选能力: 上一步被拒/失败时通知决策器换路(对应 Python on_reject)。
// PlannerLLM 实现它做动态重规划; MockLLM 不实现, 内核用类型断言探测。
type Rejecter interface {
	OnReject()
}

// Reflector —— 可选能力(结构化反思, 对应 Reflexion/RedAgent): 动作失败/被拒时,
// 内核把"哪个动作 + 失败原因"回传给决策器, 供它记住教训并在后续决策中主动避开。
// 与 Rejecter 的区别: Rejecter 只通知"换路"; OnFailure 携带具体动作与原因, 可做精确反思。
// MockLLM/PlannerLLM 可不实现(接口是可选能力); 内核用类型断言探测, 未实现则静默跳过。
type Reflector interface {
	OnFailure(action Action, reason string)
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
