// Package llm —— LLM 决策器的具体实现: MockLLM(离线脚本) 与 ClaudeLLM(真实模型)。
// PlannerLLM(确定性规划器)在 planner 包, 也实现 core.LLM 接口。
package llm

import (
	"fmt"
	"strings"

	"github.com/Coff0xc/vero/internal/core"
)

// MockLLM —— 离线自检/回退模式: 按脚本出招, 实现完整的 LLM 接口集。
// 脚本走完后优雅返回空计划, 不再报错。
type MockLLM struct {
	Script []core.Action
	i      int
	errMsg string // 错误消息(实现 ErrorReporter)
}

// NewMock —— 创建脚本 LLM。
func NewMock(script []core.Action) *MockLLM { return &MockLLM{Script: script} }

// Propose —— 单步决策: 按脚本顺序返回动作, 脚本走完返回 nil。
func (m *MockLLM) Propose(goal string, g *core.AttackGraph, history []core.HistoryItem) *core.Action {
	if m.i >= len(m.Script) {
		return nil
	}
	a := m.Script[m.i]
	m.i++
	return &a
}

// ProposePlan —— 多步规划: 将剩余脚本打包成一个计划, 支持原子性执行。
// 实现 Planner 接口, 让脚本模式也能享受计划级的 HITL 和停滞检测。
func (m *MockLLM) ProposePlan(goal string, g *core.AttackGraph, history []core.HistoryItem) *core.Plan {
	if m.i >= len(m.Script) {
		return nil
	}
	// 取出剩余脚本作为一次计划
	remaining := make([]core.Action, 0, len(m.Script)-m.i)
	for ; m.i < len(m.Script); m.i++ {
		a := m.Script[m.i]
		a.Rationale = fmt.Sprintf("[脚本] %s", a.Rationale)
		remaining = append(remaining, a)
	}
	if len(remaining) == 0 {
		return nil
	}
	return &core.Plan{
		Rationale: fmt.Sprintf("脚本模式: 执行 %d 个预定动作", len(remaining)),
		Actions:   remaining,
	}
}

// LastError —— 实现 ErrorReporter 接口: 仅在被显式设置错误时返回。
func (m *MockLLM) LastError() string { return m.errMsg }

// OnReject —— 实现 Rejecter 接口: 脚本模式被拒则记录错误并停止。
func (m *MockLLM) OnReject() {
	m.errMsg = "脚本动作被 HITL 拒绝, 战役停止"
}

// OnFailure —— 实现 Reflector 接口: 记录失败原因供前端展示。
func (m *MockLLM) OnFailure(action core.Action, reason string) {
	toolName := action.Tool
	if toolName == "engine" {
		// 引擎级错误: 不中断, 继续执行
		return
	}
	if strings.Contains(reason, "unknown tool") {
		m.errMsg = fmt.Sprintf("工具 %s 未注册, 脚本模式结束", toolName)
	}
}

// Reflect —— 实现 BattleReflector 接口: 脚本模式不做反思。
func (m *MockLLM) Reflect(goal string, g *core.AttackGraph, history []core.HistoryItem) string {
	return ""
}

// LastThinking —— 实现 ThinkingReporter 接口: 脚本模式无思考过程。
func (m *MockLLM) LastThinking() string { return "" }

// ShouldRetry —— 实现 Retrier 接口: 脚本模式对未知工具不重试。
func (m *MockLLM) ShouldRetry(reason string) bool {
	return strings.Contains(reason, "timeout")
}

// AdjustArgsForRetry —— 实现 Retrier 接口: 脚本模式重试参数不变。
func (m *MockLLM) AdjustArgsForRetry(action core.Action, reason string) map[string]any {
	return action.Args
}
