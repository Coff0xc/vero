package planner

import (
	"testing"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/tools"
)

func seed(g *core.AttackGraph, id, typ, label string) {
	g.UpsertNode(&core.Node{ID: id, Type: typ, Label: label})
	_, _ = g.Confirm(id, core.Evidence{Tool: "seed", Excerpt: "x"})
}

func hasType(g *core.AttackGraph, typ string) bool {
	for _, n := range g.Nodes {
		if n.Type == typ {
			return true
		}
	}
	return false
}

// demo_unit: 图上路径搜索每步正确 + avoid 换路 + 达成停。
func TestPlanStepwise(t *testing.T) {
	g := core.NewAttackGraph()
	seed(g, "host:t", "host", "t")
	seed(g, "service:t:80", "service", "http")
	if a, _ := Plan(g, "foothold", RULES, nil); a == nil || a.Tool != "web_vuln_scan" {
		t.Fatalf("首步应 web 打点, got %v", a)
	}
	seed(g, "web_shell:t", "web_shell", "t")
	if a, _ := Plan(g, "foothold", RULES, nil); a == nil || a.Tool != "fake_dump" {
		t.Fatalf("有 web_shell 后应捞凭证, got %v", a)
	}
	seed(g, "cred:x", "cred", "x")
	if a, _ := Plan(g, "foothold", RULES, nil); a == nil || (a.Tool != "psexec_smb" && a.Tool != "wmiexec") {
		t.Fatalf("有 cred 后应横向, got %v", a)
	}
	if a, _ := Plan(g, "foothold", RULES, map[string]bool{"lateral_smb": true}); a == nil || a.Tool != "wmiexec" {
		t.Fatalf("避开 SMB 应换 WMI, got %v", a)
	}
	seed(g, "foothold:t", "foothold", "t")
	if a, _ := Plan(g, "foothold", RULES, nil); a != nil {
		t.Fatal("达成应停")
	}
}

func e2eReg() *tools.Registry {
	r := tools.NewRegistry()
	tools.RegisterBuiltins(r) // fake_scan
	r.Register(&tools.Tool{Name: "web_vuln_scan", Level: tools.LevelScan,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "CVE-2021-x RCE 命中"} }})
	r.Register(&tools.Tool{Name: "fake_dump", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "corp\\admin:hash"} }})
	return r
}

// demo_e2e: 规划端到端驱动 agent 跑通跨场景链(侦察→web→凭证→横向)。
func TestPlanE2E(t *testing.T) {
	reg := e2eReg()
	reg.Register(&tools.Tool{Name: "psexec_smb", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "SMB shell obtained"} }})
	reg.Register(&tools.Tool{Name: "wmiexec", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "WMI shell obtained"} }})
	g, trace := core.RunAgent("拿下内网 foothold", NewPlanner("foothold"), reg, core.AutoApprove, core.DiscardEmit, 15)
	if !hasType(g, "foothold") {
		t.Fatal("规划应端到端跑通到 foothold")
	}
	if v := core.VerifyEvidence(g, trace); len(v) != 0 {
		t.Fatalf("全程证据应可逐字回查: %v", v)
	}
}

// demo_replan: SMB 横向失败 → 自动换 WMI 备选路径 → 达成 foothold。
func TestPlanReplan(t *testing.T) {
	reg := e2eReg()
	reg.Register(&tools.Tool{Name: "psexec_smb", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: false, Stderr: "SMB signing required"} }})
	reg.Register(&tools.Tool{Name: "wmiexec", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "WMI shell obtained"} }})
	g, _ := core.RunAgent("拿下 foothold", NewPlanner("foothold"), reg, core.AutoApprove, core.DiscardEmit, 15)
	if !hasType(g, "foothold") {
		t.Fatal("SMB 横向失败后应换 WMI 达成 foothold")
	}
}
