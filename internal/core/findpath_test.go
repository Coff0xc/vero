package core

import (
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// chainLLM —— 测试内确定性决策器: 依次吐动作(含 produces)。
type chainLLM struct {
	acts []Action
	i    int
}

func (c *chainLLM) Propose(_ string, _ *AttackGraph, _ []HistoryItem) *Action {
	if c.i >= len(c.acts) {
		return nil
	}
	a := c.acts[c.i]
	c.i++
	return &a
}

// produces 链端到端: parser 出 service, 规划步 produces web_shell/cred/foothold。
// 内核应把 produces 节点连成 service→web_shell→cred→foothold 的真实边, FindPath 能算出主路径。
func TestFindPathProducesChain(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&tools.Tool{Name: "fake_scan", Level: tools.LevelScan,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "Host 10.0.0.5 is up\n80/tcp open http"} },
		Parse: tools.ParseNmap})
	reg.Register(&tools.Tool{Name: "fake_web", Level: tools.LevelScan,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "web ok"} }})
	reg.Register(&tools.Tool{Name: "fake_dump", Level: tools.LevelCred,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "admin:hash123"} }})
	reg.Register(&tools.Tool{Name: "fake_lat", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "shell via SMB"} }})

	g, _ := RunAgent("x", &chainLLM{acts: []Action{
		{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}},
		{Tool: "fake_web", Args: map[string]any{"target": "10.0.0.5"}, Produces: "web_shell"},
		{Tool: "fake_dump", Args: map[string]any{"target": "10.0.0.5"}, Produces: "cred"},
		{Tool: "fake_lat", Args: map[string]any{"target": "10.0.0.5"}, Produces: "foothold"},
	}}, reg, AutoApprove, DiscardEmit, 10)

	// produces 边: service -produces-> web_shell -produces-> cred -produces-> foothold
	edges := map[string]bool{}
	for _, e := range g.Edges {
		edges[e.Src+"|"+e.Rel+"|"+e.Dst] = true
	}
	for _, want := range []string{
		"service:10.0.0.5:80|produces|web_shell:10.0.0.5",
		"web_shell:10.0.0.5|produces|cred:10.0.0.5",
		"cred:10.0.0.5|produces|foothold:10.0.0.5",
	} {
		if !edges[want] {
			t.Fatalf("缺 produces 边 %s, got %v", want, edges)
		}
	}

	wantPath := []string{"service:10.0.0.5:80", "web_shell:10.0.0.5", "cred:10.0.0.5", "foothold:10.0.0.5"}
	if p := g.FindPath("service:10.0.0.5:80", "foothold"); !samePath(p, wantPath) {
		t.Fatalf("FindPath 到 foothold 不符: got %v want %v", p, wantPath)
	}
	if p := g.FindPath("service:10.0.0.5:80", "cred"); p == nil || len(p) != 3 {
		t.Fatalf("FindPath 到 cred 应 3 节点: %v", p)
	}
	if g.FindPath("service:10.0.0.5:80", "nosuchtype") != nil {
		t.Fatal("目标类型不存在应返回 nil")
	}
}

// FindPath 只沿 confirmed 边、且两端 confirmed 节点走; hypothesis 边/节点不进主路径。
func TestFindPathConfirmedOnly(t *testing.T) {
	g := NewAttackGraph()
	g.UpsertNode(&Node{ID: "host:h", Type: "host", Label: "h"})
	g.UpsertNode(&Node{ID: "svc:s", Type: "service", Label: "s"})
	g.UpsertNode(&Node{ID: "ft:f", Type: "foothold", Label: "f"})
	// hypothesis 边不计入
	g.Edges = append(g.Edges, &Edge{Src: "svc:s", Rel: "produces", Dst: "ft:f", State: StateHypothesis})
	if p := g.FindPath("svc:s", "foothold"); p != nil {
		t.Fatalf("hypothesis 边不应计入主路径, got %v", p)
	}
	// 端点未 confirmed 也不计入
	g.Edges = append(g.Edges, &Edge{Src: "svc:s", Rel: "produces", Dst: "ft:f", State: StateConfirmed})
	if p := g.FindPath("svc:s", "foothold"); p != nil {
		t.Fatalf("端点未 confirmed 不应计入主路径, got %v", p)
	}
}

func samePath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
