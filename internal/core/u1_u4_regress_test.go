package core

import (
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// TestU4ConsecFailStall —— U4 回归: 连续失败达阈值必须停止空转, 不跑满预算。
// 此前失败路径在停滞检测前 return, `failStreak>=3` 兜底是死代码, 连续失败会耗尽预算。
func TestU4ConsecFailStall(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&tools.Tool{Name: "always_fail", Level: tools.LevelScan,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: false} }})

	script := make([]Action, 8)
	for i := range script {
		script[i] = Action{Tool: "always_fail", Args: map[string]any{"target": "10.0.0.9"}, Rationale: "retry"}
	}
	llm := &scriptLLM{script: script}
	RunAgent("probe", llm, reg, AutoApprove, DiscardEmit, 8)

	// failStreakStall=3: 连续失败应在远少于预算(8)时触发停滞退出。
	if llm.i >= 8 {
		t.Fatalf("连续失败未触发停滞: Propose 被调 %d 次(跑满预算), U4 兜底应提前停止", llm.i)
	}
	if llm.i > failStreakStall+1 {
		t.Errorf("停滞触发过晚: Propose 调用 %d 次, 期望约 %d 次", llm.i, failStreakStall)
	}
}

// TestU1NaturalLanguageClaimConfirmedByLaterAction —— U1 回归(过严修复):
// 自然语言 claim(label 不含 host 字面量)在后续同 host 动作成功时应被自动 confirm。
func TestU1NaturalLanguageClaimConfirmedByLaterAction(t *testing.T) {
	reg := testReg()
	reg.Register(&tools.Tool{Name: "recon", Level: tools.LevelRecon,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "banner: OpenSSH 7.4"} }})
	reg.Register(&tools.Tool{Name: "probe_svc", Level: tools.LevelRecon,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "22/tcp open ssh confirmed"} }})

	g, _ := RunAgent("recon", &scriptLLM{script: []Action{
		// claim 文本是纯自然语言, 不含 "10.0.0.5" 字面量; 但目标 host 是 10.0.0.5。
		{Tool: "recon", Args: map[string]any{"target": "10.0.0.5"}, Claim: "目标开放 SSH 服务", Rationale: "1"},
		// 后续【不同】动作命中同一 host 且成功 -> 独立佐证, confirm 上面的 claim。
		{Tool: "probe_svc", Args: map[string]any{"target": "10.0.0.5"}, Rationale: "2"},
	}}, reg, AutoApprove, DiscardEmit, 5)

	n := g.Nodes["claim:目标开放 SSH 服务"]
	if n == nil {
		t.Fatal("claim 节点应存在")
	}
	if n.State != StateConfirmed {
		t.Fatalf("自然语言 claim 应被后续同 host 动作 confirm, got %q", n.State)
	}
}

// TestU1SelfAssertionStaysHypothesis —— U1 回归(过松修复 + 抗幻觉不变式):
// 动作创建的 claim 不能由本动作自身 confirm(self-assertion ≠ verification)。
func TestU1SelfAssertionStaysHypothesis(t *testing.T) {
	reg := testReg()
	reg.Register(&tools.Tool{Name: "dump", Level: tools.LevelCred,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "admin:hash 10.0.0.5"} }})

	g, _ := RunAgent("提权", &scriptLLM{script: []Action{
		// 唯一动作: 创建 claim 且成功, 无后续独立验证 -> 必须保持 hypothesis。
		{Tool: "dump", Args: map[string]any{"target": "10.0.0.5"}, Claim: "10.0.0.5 存在弱口令", Rationale: "1"},
	}}, reg, AutoApprove, DiscardEmit, 5)

	n := g.Nodes["claim:10.0.0.5 存在弱口令"]
	if n == nil {
		t.Fatal("claim 节点应存在")
	}
	if n.State != StateHypothesis {
		t.Fatalf("自证 claim 应保持 hypothesis(哪怕 label 含 host), got %q", n.State)
	}
}
