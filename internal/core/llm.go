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

// Observer —— 可选能力(LLM-as-parser, 黑盒智能渗透的核心): 工具执行成功后,
// 若该工具没有固定 parser(Parse==nil)或固定 parser 产出 0 条观察, 内核把原始 stdout
// 交给决策器, 让它用语言理解力提取结构化 observation。
// 证据约束不变: 每条 observation 的 Excerpt 必须逐字存在于 stdout(内核回查),
// 防 LLM 凭空编造 —— 这是"LLM 理解"与"防幻觉底线"的接缝。
type Observer interface {
	Observe(tool string, args map[string]any, stdout string) []tools.Observation
}

// BattleReflector —— 可选能力(战役级反思): 主循环每 reflectEvery 步调用一次,
// 决策器总结"哪些假设被证伪/什么模式有效/下一步换什么方向", 返回反思文本
// (决策器自行缓存并注入下轮 prompt; 返回空串表示无输出)。
// 与 Reflector(单动作失败教训)互补: 这是粗粒度战略反思, 把探索式搜索从"盲目试"
// 变为"基于证伪逐步收敛"。
type BattleReflector interface {
	Reflect(goal string, g *AttackGraph, history []HistoryItem) string
}

// ThinkingReporter —— 可选能力(深度思考): 决策器最近一次决策的思维链
// (reasoning_content), 内核每轮决策后广播 thinking 事件, 前端折叠展示思考过程。
type ThinkingReporter interface {
	LastThinking() string
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

// Retrier —— 可选能力(修复 L1/L2: 接入 ReflexionEnhanced 的重试逻辑):
// 工具失败后, 内核询问决策器是否可自动重试(网络超时等可恢复失败),
// 以及如何调整参数重试(增加 timeout/retry 次数)。
// DeepSeekLLM 实现它(包装 llm.ShouldRetry / llm.AdjustArgsForRetry)。
type Retrier interface {
	ShouldRetry(reason string) bool
	AdjustArgsForRetry(action Action, reason string) map[string]any
}
