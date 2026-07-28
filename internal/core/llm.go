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

// HistoryItem —— 一条历史(动作 + 结果), 供决策器参考。
type HistoryItem struct {
	Outcome string // done / rejected
	Action  Action
	Result  *tools.ToolResult
}
