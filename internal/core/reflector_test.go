package core

import (
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// reflectorLLM —— 实现 core.LLM + core.Reflector 的测试决策器: 记录每次 OnFailure 回调。
// 验证内核在失败/被拒分支正确触发结构化反思(Reflexion/RedAgent 的教训注入)。
type reflectorLLM struct {
	script []Action
	i      int
	fails  []string
}

func (m *reflectorLLM) Propose(_ string, _ *AttackGraph, _ []HistoryItem) *Action {
	if m.i >= len(m.script) {
		return nil
	}
	a := m.script[m.i]
	m.i++
	return &a
}

func (m *reflectorLLM) OnFailure(a Action, reason string) {
	m.fails = append(m.fails, a.Tool+" => "+reason)
}

// 三个失败/被拒分支(未知工具 / 工具执行失败 / HITL 拒绝)都应触发 Reflector.OnFailure,
// 且携带具体动作 + 原因(原因来自 stderr 首行 / 被拒标记)。
func TestReflectorOnFailureWired(t *testing.T) {
	reg := testReg()
	reg.Register(&tools.Tool{Name: "fail_tool", Level: tools.LevelScan,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: false, Stderr: "boom"}}})
	reg.Register(&tools.Tool{Name: "exploit_a", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "shell"}}})

	llm := &reflectorLLM{script: []Action{
		{Tool: "no_such_tool", Args: map[string]any{"target": "10.0.0.5"}},
		{Tool: "fail_tool", Args: map[string]any{"target": "10.0.0.5"}},
		{Tool: "exploit_a", Args: map[string]any{"target": "10.0.0.5"}},
	}}
	RunAgent("x", llm, reg, func(_ Action, _ int) bool { return false }, DiscardEmit, 5)

	if len(llm.fails) != 3 {
		t.Fatalf("三个失败/被拒分支都应触发 OnFailure, got %d: %v", len(llm.fails), llm.fails)
	}
	found := map[string]bool{}
	for _, f := range llm.fails {
		found[f] = true
	}
	if !found["no_such_tool => unknown tool: no_such_tool"] {
		t.Fatalf("未知工具分支应回传原因: %v", llm.fails)
	}
	if !found["fail_tool => boom"] {
		t.Fatalf("工具失败分支应回传 stderr 首行原因: %v", llm.fails)
	}
	if !found["exploit_a => 未通过人工审批(HITL 拒绝)"] {
		t.Fatalf("HITL 拒绝分支应回传原因: %v", llm.fails)
	}
}

// 成功动作不应触发 OnFailure(反思只针对失败/被拒)。
func TestReflectorNotCalledOnSuccess(t *testing.T) {
	reg := testReg()
	llm := &reflectorLLM{script: []Action{
		{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}},
	}}
	RunAgent("x", llm, reg, AutoApprove, DiscardEmit, 5)
	if len(llm.fails) != 0 {
		t.Fatalf("成功动作不应触发 OnFailure, got %v", llm.fails)
	}
}
