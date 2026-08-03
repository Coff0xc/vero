package core

import (
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// scriptLLM —— 测试内的确定性决策器(不引 llm 包, 避免 core<->llm 测试期循环依赖)。
type scriptLLM struct {
	script []Action
	i      int
}

func (s *scriptLLM) Propose(_ string, _ *AttackGraph, _ []HistoryItem) *Action {
	if s.i >= len(s.script) {
		return nil
	}
	a := s.script[s.i]
	s.i++
	return &a
}

func testReg() *tools.Registry {
	r := tools.NewRegistry()
	tools.RegisterBuiltins(r)
	return r
}

// 1) parser 驱动图 + 证据约束: host/service 节点、runs 边, 全程证据可回查。
func TestParserDrivesGraph(t *testing.T) {
	g, trace := RunAgent("拿下 10.0.0.5",
		&scriptLLM{script: []Action{{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}, Rationale: "存活扫描"}}},
		testReg(), AutoApprove, DiscardEmit, 5)
	if _, ok := g.Nodes["host:10.0.0.5"]; !ok {
		t.Fatal("应有 host 节点")
	}
	svc, ok := g.Nodes["service:10.0.0.5:22"]
	if !ok {
		t.Fatal("parser 应提取 service 节点")
	}
	if svc.State != StateConfirmed {
		t.Fatalf("service 应 confirmed, got %s", svc.State)
	}
	hasRuns := false
	for _, e := range g.Edges {
		if e.Rel == "runs" {
			hasRuns = true
		}
	}
	if !hasRuns {
		t.Fatal("应有 host-runs->service 边")
	}
	if v := VerifyEvidence(g, trace); len(v) != 0 {
		t.Fatalf("正常流程不应有证据违规: %v", v)
	}
}

// 2) 证据逐字回查: 注入编造证据应被抓到。
func TestEvidenceVerbatimCatchesHallucination(t *testing.T) {
	g, trace := RunAgent("x",
		&scriptLLM{script: []Action{{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}}}},
		testReg(), AutoApprove, DiscardEmit, 5)
	n := g.Nodes["host:10.0.0.5"]
	n.Evidence = append(n.Evidence, Evidence{Tool: "x", Excerpt: "编造的证据XYZ"})
	if v := VerifyEvidence(g, trace); len(v) == 0 {
		t.Fatal("幻觉证据应被逐字回查抓到")
	}
}

// 3) claim 即验证: 声称默认 hypothesis, 独立验证后 confirmed。
func TestClaimVerifyPromotes(t *testing.T) {
	reg := testReg()
	reg.Register(&tools.Tool{Name: "fake_dump", Level: tools.LevelCred,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "admin:hash123"} }})
	reg.Register(&tools.Tool{Name: "verify_cred", Level: tools.LevelCred,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "auth OK as admin"} }})
	g, _ := RunAgent("提权", &scriptLLM{script: []Action{
		{Tool: "fake_dump", Args: map[string]any{"target": "10.0.0.5"}, Rationale: "dump", Claim: "cred:admin"},
		{Tool: "verify_cred", Args: map[string]any{"target": "10.0.0.5", "verifies": "cred:admin"}, Rationale: "验证"},
	}}, reg, AutoApprove, DiscardEmit, 5)
	if g.Nodes["claim:cred:admin"].State != StateConfirmed {
		t.Fatal("验证后 claim 应 confirmed")
	}
}

// 4) claim 未验证 -> 保持 hypothesis(不可信)。
func TestUnverifiedClaimStaysHypothesis(t *testing.T) {
	reg := testReg()
	reg.Register(&tools.Tool{Name: "fake_dump", Level: tools.LevelCred,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "admin:hash"} }})
	g, _ := RunAgent("提权",
		&scriptLLM{script: []Action{{Tool: "fake_dump", Args: map[string]any{"target": "10.0.0.5"}, Claim: "cred:x"}}},
		reg, AutoApprove, DiscardEmit, 5)
	if g.Nodes["claim:cred:x"].State != StateHypothesis {
		t.Fatal("未验证 claim 应保持 hypothesis")
	}
}

// 5) HITL: L3+ 动作必须走审批钩子。
func TestHITLGatesExploit(t *testing.T) {
	reg := testReg()
	reg.Register(&tools.Tool{Name: "fake_exploit", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "shell"} }})
	var seen []int
	RunAgent("x", &scriptLLM{script: []Action{{Tool: "fake_exploit", Args: map[string]any{"target": "t"}, Rationale: "提权"}}},
		reg, func(_ Action, l int) bool { seen = append(seen, l); return true }, DiscardEmit, 3)
	if len(seen) != 1 || seen[0] != 3 {
		t.Fatalf("L3 动作应触发 HITL 审批, got %v", seen)
	}
}

// 6) 证据约束: 无证据 confirm 必须被拒。
func TestEvidenceConstraintRejectsEmpty(t *testing.T) {
	g := NewAttackGraph()
	g.UpsertNode(&Node{ID: "host:x", Type: "host", Label: "x"})
	if _, err := g.Confirm("host:x", Evidence{}); err == nil {
		t.Fatal("无证据 confirm 应抛错")
	}
}

// planLLM —— 测试内的多步规划决策器(实现 Planner 接口, 每次给一段计划, 耗尽返回 nil)。
type planLLM struct {
	plans []*Plan
	i     int
}

func (p *planLLM) Propose(_ string, _ *AttackGraph, _ []HistoryItem) *Action { return nil }

func (p *planLLM) ProposePlan(_ string, _ *AttackGraph, _ []HistoryItem) *Plan {
	if p.i >= len(p.plans) {
		return nil
	}
	pl := p.plans[p.i]
	p.i++
	return pl
}

// 7) 多步计划: 按序执行整段计划; 某步失败 -> 中断后续步(依赖链)。
func TestPlanExecutesInOrderAndAbortsOnFailure(t *testing.T) {
	reg := testReg()
	var executed []string
	mark := func(name string) func(map[string]any) tools.ToolResult {
		return func(map[string]any) tools.ToolResult {
			executed = append(executed, name)
			return tools.ToolResult{Success: true, Stdout: "ok: " + name}
		}
	}
	reg.Register(&tools.Tool{Name: "fail_tool", Level: tools.LevelScan,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: false, Stderr: "boom"} }})
	reg.Register(&tools.Tool{Name: "never_tool", Level: tools.LevelScan, Run: mark("never_tool")})

	g, _ := RunAgent("x", &planLLM{plans: []*Plan{{Rationale: "链", Actions: []Action{
		{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}},
		{Tool: "fail_tool", Args: map[string]any{"target": "10.0.0.5"}},
		{Tool: "never_tool", Args: map[string]any{"target": "10.0.0.5"}},
	}}}}, reg, AutoApprove, DiscardEmit, 5)

	for _, name := range executed {
		if name == "never_tool" {
			t.Fatal("计划中途失败后, 后续步骤不应执行(依赖链中断)")
		}
	}
	if _, ok := g.Nodes["host:10.0.0.5"]; !ok {
		t.Fatal("计划首步(成功)应更新攻击图")
	}
}

// 8) 多步计划: HITL 拒绝 -> 中断剩余步骤, 且拒绝动作不入图。
func TestPlanHitlDenyAbortsRemaining(t *testing.T) {
	reg := testReg()
	reg.Register(&tools.Tool{Name: "exploit_a", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "shell-a"} }})
	reg.Register(&tools.Tool{Name: "exploit_b", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "shell-b"} }})
	var ran []string

	g, _ := RunAgent("x", &planLLM{plans: []*Plan{{Rationale: "双利用", Actions: []Action{
		{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}},
		{Tool: "exploit_a", Args: map[string]any{"target": "10.0.0.5"}, Produces: "web_shell"},
		{Tool: "exploit_b", Args: map[string]any{"target": "10.0.0.5"}, Produces: "foothold"},
	}}}}, reg, func(_ Action, l int) bool {
		ran = append(ran, "hitl")
		return false // 拒绝 exploit_a
	}, DiscardEmit, 5)

	if len(ran) != 1 {
		t.Fatalf("只应触发一次 HITL(拒绝后计划中断), got %d", len(ran))
	}
	for _, id := range g.Order {
		if g.Nodes[id].Type == "foothold" {
			t.Fatal("被拒计划的后续步骤不应产出节点")
		}
	}
}

// 9) 计划中断后: 决策器可再次给出新计划(动态重规划), 战役继续推进。
func TestPlanCanReproposeAfterAbort(t *testing.T) {
	reg := testReg()
	reg.Register(&tools.Tool{Name: "fail_tool", Level: tools.LevelScan,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: false, Stderr: "boom"} }})

	// 第一段计划: 扫描成功、fail_tool 失败(中断); 第二段计划: 重规划后只扫描。
	llm := &planLLM{plans: []*Plan{
		{Rationale: "第一段", Actions: []Action{
			{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}},
			{Tool: "fail_tool", Args: map[string]any{"target": "10.0.0.5"}},
		}},
		{Rationale: "重规划", Actions: []Action{
			{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}},
		}},
	}}
	var steps []string
	g, _ := RunAgent("x", llm, reg, AutoApprove, func(e Event) {
		if e.Kind == "step" {
			steps = append(steps, e.Data["tool"].(string))
		}
	}, 5)

	if len(steps) != 3 || steps[2] != "fake_scan" {
		t.Fatalf("计划中断后应重规划并继续执行, 轨迹应为 [fake_scan fail_tool fake_scan], got %v", steps)
	}
	if _, ok := g.Nodes["host:10.0.0.5"]; !ok {
		t.Fatal("重规划后的执行应继续更新攻击图")
	}
}
