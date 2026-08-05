package llm

import (
	"testing"

	"github.com/Coff0xc/vero/internal/core"
)

// mockProposeOnly —— 只实现 Propose(无 Planner 能力), 触发 targetInjector 回退路径。
type mockProposeOnly struct{}

func (m *mockProposeOnly) Propose(goal string, g *core.AttackGraph, h []core.HistoryItem) *core.Action {
	return &core.Action{Tool: "port_scan", Args: map[string]any{}}
}

// TestInjectorPlanFallbackInjectsTarget —— D31 回归: 内层不支持 Planner 时,
// ProposePlan 回退路径也必须给动作注入 target(经 t.Propose 的 injectTarget)。
func TestInjectorPlanFallbackInjectsTarget(t *testing.T) {
	ti := WithTarget(&mockProposeOnly{}, "http://file.nciyuan.net:8080").(*targetInjector)
	ti.inner = &mockProposeOnly{}

	p := ti.ProposePlan("test", core.NewAttackGraph(), nil)
	if p == nil {
		t.Fatal("回退路径应返回单步计划")
	}
	if len(p.Actions) != 1 {
		t.Fatalf("回退路径应只有 1 个动作, got %d", len(p.Actions))
	}
	got, ok := p.Actions[0].Args["target"].(string)
	if !ok || got != "http://file.nciyuan.net:8080" {
		t.Errorf("回退路径动作 target 未注入, got %q", got)
	}
}

// TestInjectorPlanPassthroughInjectsTarget —— Planner 路径: 计划里每个动作都注入 target。
type mockPlanner struct{}

func (m *mockPlanner) Propose(goal string, g *core.AttackGraph, h []core.HistoryItem) *core.Action {
	return nil
}
func (m *mockPlanner) ProposePlan(goal string, g *core.AttackGraph, h []core.HistoryItem) *core.Plan {
	return &core.Plan{Actions: []core.Action{
		{Tool: "http_probe", Args: map[string]any{}},
		{Tool: "web_vuln_scan", Args: map[string]any{"target": "user-specified"}},
	}}
}

func TestInjectorPlanPassthroughInjectsTarget(t *testing.T) {
	ti := WithTarget(&mockPlanner{}, "http://file.nciyuan.net:8080").(*targetInjector)
	ti.inner = &mockPlanner{}

	p := ti.ProposePlan("test", core.NewAttackGraph(), nil)
	if p == nil || len(p.Actions) != 2 {
		t.Fatalf("Planner 路径应透传 2 个动作, got %+v", p)
	}
	// 空 target 应注入; 已有 target 不得覆盖
	if got := p.Actions[0].Args["target"]; got != "http://file.nciyuan.net:8080" {
		t.Errorf("空 target 动作未注入, got %v", got)
	}
	if got := p.Actions[1].Args["target"]; got != "user-specified" {
		t.Errorf("已有 target 被覆盖, got %v", got)
	}
}
