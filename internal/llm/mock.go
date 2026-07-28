// Package llm —— LLM 决策器的具体实现: MockLLM(离线脚本) 与 ClaudeLLM(真实模型)。
// PlannerLLM(确定性规划器)在 planner 包, 也实现 core.LLM 接口。
package llm

import "redcell/internal/core"

// MockLLM —— 离线自检: 按脚本出招, 脚本走完返回 nil = 结束(对应 Python MockLLM)。
type MockLLM struct {
	Script []core.Action
	i      int
}

func NewMock(script []core.Action) *MockLLM { return &MockLLM{Script: script} }

func (m *MockLLM) Propose(goal string, g *core.AttackGraph, history []core.HistoryItem) *core.Action {
	if m.i >= len(m.Script) {
		return nil
	}
	a := m.Script[m.i]
	m.i++
	return &a
}
