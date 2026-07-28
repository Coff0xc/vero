package server

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/llm"
	"github.com/Coff0xc/vero/internal/report"
	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/tools"
)

// buildLiveRegistry —— 真实工具集: Go 原生端口扫描 + web/AD 场景包(curl/nuclei/exploit)。
func buildLiveRegistry() (*tools.Registry, *scenarios.Manager) {
	reg := tools.NewRegistry()
	reg.Register(&tools.Tool{Name: "port_scan", Level: tools.LevelScan,
		Desc: "TCP connect 端口扫描, 发现开放端口/服务(target 用 host)", Run: tools.PortScan, Parse: tools.ParseNmap})
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)
	return reg, sm
}

// RunCampaign —— 对 target 跑一场真实战役: 真实工具 + (DeepSeek/Claude 自主 或 真实工具脚本)决策,
// SSE 广播, Web HITL 门控, 落盘渗透报告。这是 Web 作战室的真实后端(不再是 mock demo)。
func (s *Server) RunCampaign(target string) {
	s.gate.CancelAll()
	if target == "" {
		target = "http://localhost:3000"
	}

	reg, sm := buildLiveRegistry()

	var chosen core.LLM
	engine := "脚本(无 key)"
	switch {
	case os.Getenv("DEEPSEEK_API_KEY") != "":
		chosen = llm.WithTarget(llm.NewDeepSeek(reg), target)
		engine = "DeepSeek 自主"
	case os.Getenv("ANTHROPIC_API_KEY") != "":
		chosen = llm.WithTarget(llm.NewClaude(reg), target)
		engine = "Claude 自主"
	default:
		chosen = llm.NewMock([]core.Action{
			{Tool: "port_scan", Args: map[string]any{"target": target}, Rationale: "端口扫描"},
			{Tool: "http_probe", Args: map[string]any{"target": target}, Rationale: "HTTP 指纹"},
			{Tool: "web_vuln_scan", Args: map[string]any{"target": target}, Rationale: "漏洞扫描"},
			{Tool: "exploit_sqli", Args: map[string]any{"target": target}, Rationale: "SQLi 利用尝试"},
		})
	}

	cid, _ := s.store.StartCampaign("侦察 " + target)
	s.broker.Emit(core.Event{Kind: "engine", Data: map[string]any{"engine": engine, "target": target}})

	emit := func(e core.Event) {
		if e.Kind == "tool" { // 审计挂到 tool 事件(执行后), 回填 success —— 修 Python 原 success 恒 null 缺陷
			lvl, _ := e.Data["level"].(int)
			tn, _ := e.Data["tool"].(string)
			args, _ := e.Data["args"].(map[string]any)
			succ, _ := e.Data["success"].(bool)
			_ = s.auditor.Record(tn, args, lvl, &succ, nil)
		}
		_ = s.store.SaveEvent(cid, e)
		s.broker.Emit(e)
	}
	goal := "对目标 " + target + " 做红队侦察与漏洞验证: 端口扫描→HTTP指纹→漏扫→发现可利用点尝试利用(如 SQLi)。用真实证据坐实; 充分则停止。"
	g, trace := core.RunAgent(goal, chosen, reg, s.gate.Approve, emit, 10)

	services := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Type == "service" {
			services[strings.SplitN(n.Label, " on ", 2)[0]] = true
		}
	}
	emit(core.Event{Kind: "route", Data: map[string]any{
		"services": sortedKeys(services), "activated": sm.Route(services)}})

	conf, hypo := 0, 0
	for _, n := range g.Nodes {
		switch n.State {
		case core.StateConfirmed:
			conf++
		case core.StateHypothesis:
			hypo++
		}
	}
	viol := len(core.VerifyEvidence(g, trace))
	_ = s.store.SaveSnapshot(cid, g)
	_ = s.store.EndCampaign(cid, conf, hypo, viol)

	reportFile := fmt.Sprintf("redcell-report-%d.md", cid)
	md := report.Markdown(target, g, viol, time.Now().Format("2006-01-02 15:04:05"))
	_ = os.WriteFile(reportFile, []byte(md), 0o644)

	emit(core.Event{Kind: "summary", Data: map[string]any{
		"confirmed": conf, "hypothesis": hypo, "evidence_violations": viol, "report": reportFile}})
	emit(core.Event{Kind: "done", Data: map[string]any{"reason": "战役结束, 报告已生成: " + reportFile}})
}

func sortedKeys(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
