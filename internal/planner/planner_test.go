package planner

import (
	"strings"
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
// 用仿真规则集 RULES(仅测试用); Plan 的 blocked 参数传 nil。
func TestPlanStepwise(t *testing.T) {
	g := core.NewAttackGraph()
	seed(g, "host:t", "host", "t")
	seed(g, "service:t:80", "service", "http")
	if a, _ := Plan(g, "foothold", RULES, nil, nil); a == nil || a.Tool != "web_vuln_scan" {
		t.Fatalf("首步应 web 打点, got %v", a)
	}
	seed(g, "web_shell:t", "web_shell", "t")
	if a, _ := Plan(g, "foothold", RULES, nil, nil); a == nil || a.Tool != "fake_dump" {
		t.Fatalf("有 web_shell 后应捞凭证, got %v", a)
	}
	seed(g, "cred:x", "cred", "x")
	if a, _ := Plan(g, "foothold", RULES, nil, nil); a == nil || (a.Tool != "nxc_smb_spray" && a.Tool != "nxc_wmi_exec") {
		t.Fatalf("有 cred 后应横向, got %v", a)
	}
	if a, _ := Plan(g, "foothold", RULES, map[string]bool{"lateral_smb": true}, nil); a == nil || a.Tool != "nxc_wmi_exec" {
		t.Fatalf("避开 SMB 应换 WMI, got %v", a)
	}
	seed(g, "foothold:t", "foothold", "t")
	if a, _ := Plan(g, "foothold", RULES, nil, nil); a != nil {
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
// 仿真工具显式注册(NewSimPlanner + e2eReg)。
func TestPlanE2E(t *testing.T) {
	reg := e2eReg()
	reg.Register(&tools.Tool{Name: "nxc_smb_spray", Level: tools.LevelCred,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "SMB spray OK"} }})
	reg.Register(&tools.Tool{Name: "nxc_wmi_exec", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "WMI shell obtained"} }})
	g, trace := core.RunAgent("拿下内网 foothold", NewSimPlanner("foothold"), reg, core.AutoApprove, core.DiscardEmit, 15)
	if !hasType(g, "foothold") {
		t.Fatal("规划应端到端跑通到 foothold")
	}
	if v := core.VerifyEvidence(g, trace); len(v) != 0 {
		t.Fatalf("全程证据应可逐字回查: %v", v)
	}
}

// demo_replan: SMB 横向失败 → 自动换 WMI 备选路径 → 达成 foothold。
// 现在内核会把失败(action+reason)经 Reflector.OnFailure 回传 PlannerLLM,
// 精确避让 psexec_smb 工具, 而非仅避规则名。
func TestPlanReplan(t *testing.T) {
	reg := e2eReg()
	reg.Register(&tools.Tool{Name: "nxc_smb_spray", Level: tools.LevelCred,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: false, Stderr: "SMB signing required"} }})
	reg.Register(&tools.Tool{Name: "nxc_wmi_exec", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "WMI shell obtained"} }})
	g, _ := core.RunAgent("拿下 foothold", NewSimPlanner("foothold"), reg, core.AutoApprove, core.DiscardEmit, 15)
	if !hasType(g, "foothold") {
		t.Fatal("SMB 横向失败后应换 WMI 达成 foothold")
	}
}

// demo_real: 真实模式(NewPlanner)在本机无 nmap 时必须明确停机并暴露原因,
// 绝不回退仿真“假装打”。
func TestPlannerRealNoTools(t *testing.T) {
	p := NewPlanner("foothold")
	g := core.NewAttackGraph()
	a := p.Propose("goal", g, nil)
	if a != nil {
		t.Fatalf("无 nmap 时真实模式应停机(Propose=nil), got %v", a)
	}
	if p.LastError() == "" {
		t.Fatal("停机时应暴露原因(LastError 非空)")
	}
	if !strings.Contains(p.LastError(), "nmap") {
		t.Fatalf("原因应提示安装 nmap, got %q", p.LastError())
	}
}

// demo_real: 真实模式 + 已装 nmap: 规则只引用真实工具, 无任何 fake_*。
func TestPlannerRealWithNmap(t *testing.T) {
	rules, err := MakeRules(true, false)
	if err != nil {
		t.Fatalf("有 nmap 时不应报不可用: %v", err)
	}
	for _, r := range rules {
		if strings.HasPrefix(r.Tool, "fake_") {
			t.Fatalf("真实模式规则不得引用仿真工具: %s", r.Tool)
		}
	}
	// 侦察规则必须用 nmap_scan, 凭证规则必须用真实工具(secretsdump)。
	byName := map[string]Rule{}
	for _, r := range rules {
		byName[r.Name] = r
	}
	if got := byName["recon"].Tool; got != "nmap_scan" {
		t.Fatalf("recon 应用 nmap_scan, got %q", got)
	}
	if got := byName["loot_creds"].Tool; got != "secretsdump" {
		t.Fatalf("loot_creds 应用 secretsdump, got %q", got)
	}
}

// demo_reflect: OnFailure 精确按工具避让: nxc_smb_spray 失败后, 横向路径应改走 nxc_wmi_exec。
func TestPlannerOnFailureAvoidsTool(t *testing.T) {
	rules, err := MakeRules(true, false)
	if err != nil {
		t.Fatalf("MakeRules: %v", err)
	}
	g := core.NewAttackGraph()
	seed(g, "host:t", "host", "t")
	seed(g, "service:t:80", "service", "http")
	seed(g, "web_shell:t", "web_shell", "t")
	seed(g, "cred:x", "cred", "x")

	p := &PlannerLLM{GoalType: "foothold", Rules: rules, avoid: map[string]bool{}, blocked: map[string]string{}}
	a := p.Propose("goal", g, nil)
	if a == nil || a.Tool != "nxc_smb_spray" {
		t.Fatalf("首步横向应 nxc_smb_spray, got %v", a)
	}
	// 内核在失败时调 OnFailure(action, reason)
	p.OnFailure(core.Action{Tool: "nxc_smb_spray"}, "SMB signing required")
	a2 := p.Propose("goal", g, nil)
	if a2 == nil || a2.Tool != "nxc_wmi_exec" {
		t.Fatalf("nxc_smb_spray 失败后应改走 nxc_wmi_exec, got %v", a2)
	}
	// 两条横向工具都失败 -> 无路可走, 暴露原因
	p.OnFailure(core.Action{Tool: "nxc_wmi_exec"}, "WMI blocked")
	a3 := p.Propose("goal", g, nil)
	if a3 != nil {
		t.Fatalf("全部横向工具失败后应停机, got %v", a3)
	}
	if p.LastError() == "" {
		t.Fatal("停机时应暴露失败工具原因")
	}
	if !strings.Contains(p.LastError(), "nxc_smb_spray") {
		t.Fatalf("原因应含失败工具名, got %q", p.LastError())
	}
}
