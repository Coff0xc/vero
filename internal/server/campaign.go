package server

import (
	"context"
	"encoding/json"
	"net/http"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/config"
	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/llm"
	"github.com/Coff0xc/vero/internal/report"
	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/tools"
)

// scriptLLM —— 脚本模式决策器(无 key 时的确定性动作序列)。
func scriptLLM(reg *tools.Registry, target string) core.LLM {
	return llm.NewMock([]core.Action{
		{Tool: "port_scan", Args: map[string]any{"target": target}, Rationale: "端口扫描"},
		{Tool: "http_probe", Args: map[string]any{"target": target}, Rationale: "HTTP 指纹"},
		{Tool: "web_vuln_scan", Args: map[string]any{"target": target}, Rationale: "漏洞扫描"},
		{Tool: "exploit_sqli", Args: map[string]any{"target": target}, Rationale: "SQLi 利用尝试"},
	})
}

// defaultModelFor —— 按 provider ID 推默认模型名(用户设了 key 但没选模型时兜底)。
func defaultModelFor(id string) string {
	switch id {
	case "deepseek":
		return "deepseek-chat"
	case "openai":
		return "gpt-4o-mini"
	case "kimi":
		return "moonshot-v1-8k"
	case "qwen":
		return "qwen-turbo"
	case "openrouter":
		return "openrouter-chat"
	case "ollama":
		return "" // ollama 由 /api/tags 自发现, 空可接受
	case "zhipu":
		return "glm-4-flash"
	default:
		return ""
	}
}

// buildLiveRegistry —— 真实工具集: Go 原生端口扫描 + web/AD 场景包(curl/nuclei/exploit)。
func buildLiveRegistry() (*tools.Registry, *scenarios.Manager) {
	reg := tools.NewRegistry()
	reg.Register(&tools.Tool{Name: "port_scan", Level: tools.LevelScan,
		Desc: "TCP connect 端口扫描, 发现开放端口/服务", Run: tools.PortScan, Parse: tools.ParseNmap,
		Args: []tools.ArgSpec{
			{Name: "target", Desc: "主机/IP(不带 scheme/端口), 如 10.0.0.5 或 localhost", Required: true},
			{Name: "ports", Desc: "端口范围, 如 1-10000 或 80,443,8080, 默认常见端口"},
		}})
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)
	return reg, sm
}

// RunCampaign —— 对 target 跑一场真实战役: 真实工具 + (DeepSeek/Claude 自主 或 真实工具脚本)决策,
// SSE 广播, Web HITL 门控, 落盘渗透报告。这是 Web 作战室的真实后端(不再是 mock demo)。
// ctx 由 handleStart 提供: 操作员点"停止"即取消, 核心循环与 HITL 等待都会立即响应。
func (s *Server) RunCampaign(ctx context.Context, target string) {
	s.gate.CancelAll()
	if target == "" {
		target = "http://localhost:3000"
	}

	reg, sm := buildLiveRegistry()

	cid, err0 := s.store.StartCampaign("侦察 " + target)
	if err0 != nil {
		s.broker.Emit(core.Event{Kind: "error", Data: map[string]any{"msg": "战役持久化失败: " + err0.Error()}})
	}

	// emit —— 事件广播闭包: 审计 + 落库 + SSE。必须在决策引擎选择前定义(引擎选择也会广播事件)。
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

	// 决策引擎: 多提供商(OpenAI 兼容)优先, 兼容旧 claude/deepseek 配置; 无 key 回退脚本。
	s.cfgMu.Lock()
	cfg := s.cfg
	s.cfgMu.Unlock()
	// YOLO 模式: 跳过全部 HITL 审批(授权靶场/自动化)。
	approve := func(a core.Action, lvl int) bool { return s.gate.ApproveCtx(ctx, a, lvl) }
	if cfg.YOLO {
		approve = core.AutoApprove
	}

	// 智能引擎选择: 有 key 就用真实模型, 空模型名用默认值兜底, 不再强制要求选模型。
	// 先选 provider(多提供商), 再兼容旧 key 路径(DeepSeek/Claude)。
	var chosen core.LLM
	engine := "脚本(无 key)"
	prov := cfg.Active()
	hasProviderKey := prov != nil && prov.APIKey != ""
	hasOldDeepSeek := cfg.DeepSeekKey != ""
	hasOldAnthropic := cfg.AnthropicKey != ""
	useProvider := hasProviderKey || (prov != nil && prov.ID == "ollama")

	// ── 诊断日志: 输出当前配置状态, 便于排障 ──
	fmt.Fprintf(os.Stderr, "[engine] config: engine=%s, active_provider=%s, prov_key=%v, prov_model=%q, old_deepseek=%v, old_anthropic=%v\n",
		cfg.Engine, cfg.ActiveProvider, hasProviderKey, func() string { if prov != nil { return prov.Model }; return "(nil)" }(),
		hasOldDeepSeek, hasOldAnthropic)

	// provider 有 key 但没选模型 → 按 ID 推默认模型名
	if useProvider && prov.Model == "" {
		prov.Model = defaultModelFor(prov.ID)
		fmt.Fprintf(os.Stderr, "[engine] auto-filled model for %s: %s\n", prov.ID, prov.Model)
	}

	switch {
	case cfg.Engine == config.EngineScript:
		chosen = scriptLLM(reg, target)
		engine = "脚本(固定)"
	case useProvider:
		chosen = llm.WithTarget(llm.NewOpenAI(prov, reg, cfg.Temperature), target)
		engine = prov.Name + " 自主"
		if cfg.DeepThinking {
			engine += "·深度思考"
		}
		fmt.Fprintf(os.Stderr, "[engine] using provider: %s (model=%s)\n", prov.Name, prov.Model)
	case hasOldDeepSeek:
		chosen = llm.WithTarget(llm.NewDeepSeek(reg, cfg.DeepSeekKey, cfg.Temperature), target)
		engine = "DeepSeek 自主"
		fmt.Fprintf(os.Stderr, "[engine] using legacy DeepSeek key (model=%s)\n", func() string { m := os.Getenv("VERO_MODEL"); if m != "" { return m }; return "deepseek-chat" }())
	case hasOldAnthropic:
		chosen = llm.WithTarget(llm.NewClaude(reg, cfg.AnthropicKey, cfg.Temperature), target)
		engine = "Claude 自主"
		fmt.Fprintf(os.Stderr, "[engine] using legacy Anthropic key\n")
	default:
		// 引擎回退通知: 用 warning 事件而不是 tool 事件, 避免前端误显示为"引擎失败"
		emit(core.Event{Kind: "warning", Data: map[string]any{"msg": "未配置可用模型 API Key, 自动回退到脚本模式执行"}})
		chosen = scriptLLM(reg, target)
		engine = "脚本(key 缺失回退)"
		fmt.Fprintf(os.Stderr, "[engine] no key available, falling back to script mode\n")
	}

	s.broker.Emit(core.Event{Kind: "engine", Data: map[string]any{"engine": engine, "target": target, "yolo": cfg.YOLO}})

	goal := "对目标 " + target + " 做红队侦察与漏洞验证: 端口扫描→HTTP指纹→漏扫→发现可利用点尝试利用(如 SQLi)。用真实证据坐实; 充分则停止。"
	budget := cfg.MaxBudget
	if budget < 1 {
		budget = 20
	}
	g, trace := core.RunAgentCtx(ctx, goal, chosen, reg, approve, emit, budget)

	services := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Type == "service" {
			services[strings.SplitN(n.Label, " on ", 2)[0]] = true
		}
	}
	emit(core.Event{Kind: "route", Data: map[string]any{
		"services": sortedKeys(services), "activated": sm.Route(services)}})

	// 主路径: 从首个 confirmed service 节点沿 confirmed 边 BFS 到 foothold(找不到再试 cred)。
	// produces/verifies 边由内核补(规划链/claim 验证), 非空则广播 path 事件供前端高亮攻击链。
	if p := mainPath(g); len(p) > 0 {
		emit(core.Event{Kind: "path", Data: map[string]any{"nodes": p}})
	}

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

	reportFile := fmt.Sprintf("vero-report-%d.md", cid)
	md := report.Markdown(target, g, viol, time.Now().Format("2006-01-02 15:04:05"))
	_ = os.WriteFile(reportFile, []byte(md), 0o644)

	// 捕获战役上下文(对话式问答感知: /api/chat 基于它回答)。
	s.captureCtx(target, engine, g)

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

// mainPath —— 主攻击路径: 从任一 confirmed service 节点出发, 沿 confirmed 边 BFS 到 foothold,
// 找不到再试 cred; 取首个非空路径(service 按插入序优先, 路径由 FindPath 保证最短)。
func mainPath(g *core.AttackGraph) []string {
	// 先找 confirmed service 节点(按插入序); 每个都试 foothold。
	var serviceIDs []string
	for _, id := range g.Order {
		if n := g.Nodes[id]; n.Type == "service" && n.State == core.StateConfirmed {
			serviceIDs = append(serviceIDs, id)
		}
	}
	for _, sid := range serviceIDs {
		if p := g.FindPath(sid, "foothold"); len(p) > 0 {
			return p
		}
	}
	// foothold 无路 -> 退而求其次找 cred(凭证路径)。
	for _, sid := range serviceIDs {
		if p := g.FindPath(sid, "cred"); len(p) > 0 {
			return p
		}
	}
	return nil
}

// campaignCtx —— 最近一次战役的对话感知上下文。
type campaignCtx struct {
	Target string
	Engine string
	Graph  *core.AttackGraph
}

// captureCtx —— 战役结束(或取消)时捕获上下文, 供 /api/chat 感知。
func (s *Server) captureCtx(target, engine string, g *core.AttackGraph) {
	s.ctxMu.Lock()
	s.lastCtx = &campaignCtx{Target: target, Engine: engine, Graph: g}
	s.ctxMu.Unlock()
}

// campaignContext —— 把最近战役渲染成问答上下文(confirmed 发现 + severity + 证据 excerpt)。
func (s *Server) campaignContext() string {
	s.ctxMu.Lock()
	lc := s.lastCtx
	s.ctxMu.Unlock()
	if lc == nil || lc.Graph == nil {
		return "(当前无战役数据 — 先发起一次渗透, 或询问通用安全知识)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "目标: %s\n引擎: %s\n", lc.Target, lc.Engine)
	b.WriteString("\n已确认发现(confirmed, 证据逐字可查):\n")
	n := 0
	for _, id := range lc.Graph.Order {
		node := lc.Graph.Nodes[id]
		if node.State != core.StateConfirmed {
			continue
		}
		n++
		fmt.Fprintf(&b, "- %s (%s)", id, node.Type)
		if node.Severity != "" {
			fmt.Fprintf(&b, " severity=%s", node.Severity)
		}
		if node.Technique != "" {
			fmt.Fprintf(&b, " ATT&CK=%s", node.Technique)
		}
		if len(node.Evidence) > 0 {
			fmt.Fprintf(&b, " 证据[%s]: %s", node.Evidence[0].Tool, tools.Clip(node.Evidence[0].Excerpt, 90))
		}
		b.WriteString("\n")
		if n >= 25 {
			b.WriteString("...\n")
			break
		}
	}
	if n == 0 {
		b.WriteString("(无 — 战役可能未发现漏洞或已失败)\n")
	}
	// 待验证假设(供回答时区分)。
	hypo := 0
	for _, id := range lc.Graph.Order {
		if lc.Graph.Nodes[id].State == core.StateHypothesis {
			hypo++
		}
	}
	fmt.Fprintf(&b, "\n待验证假设: %d 条\n", hypo)
	return b.String()
}

// sanitizeHistory —— 对话历史卫生(修 D17): 只保留 user/assistant 角色, 拒绝伪造 role;
// 限制条目数(20)与单条长度(2000 字符), 防 prompt 撑爆与注入。
func sanitizeHistory(history [][2]string) [][2]string {
	const (
		maxItems = 20
		maxLen   = 2000
	)
	out := make([][2]string, 0, len(history))
	for _, h := range history {
		if h[0] != "user" && h[0] != "assistant" {
			continue // 伪造 system/工具等角色一律丢弃
		}
		r := []rune(strings.TrimSpace(h[1]))
		if len(r) == 0 {
			continue
		}
		if len(r) > maxLen {
			r = r[:maxLen]
		}
		out = append(out, [2]string{h[0], string(r)})
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

// handleChat —— 对话式问答: 感知当前战役上下文, 用配置的 LLM 回答用户问题。
// 请求: {question, history?[[role,content]...]}; 响应: {answer}。
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Question string     `json:"question"`
		History  [][2]string `json:"history"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.TrimSpace(body.Question) == "" {
		http.Error(w, "question required", http.StatusBadRequest)
		return
	}
	body.History = sanitizeHistory(body.History) // D17: 拒绝伪造 role, 限制条目/长度
	ctx := s.campaignContext()
	reg := tools.NewRegistry()
	s.cfgMu.Lock()
	cfg := s.cfg
	s.cfgMu.Unlock()
	var chat llm.Chatter
	var thinking string
	var oai *llm.OpenAILLM

	// ── 统一引擎选择(与 RunCampaign 保持一致) ──
	prov := cfg.Active()
	hasProviderKey := prov != nil && prov.APIKey != ""
	hasOldDeepSeek := cfg.DeepSeekKey != ""
	hasOldAnthropic := cfg.AnthropicKey != ""
	useProvider := hasProviderKey || (prov != nil && prov.ID == "ollama")

	if useProvider {
		model := prov.Model
		if model == "" {
			model = defaultModelFor(prov.ID)
		}
		tmpProv := *prov
		tmpProv.Model = model
		oai = llm.NewOpenAI(&tmpProv, reg, cfg.Temperature)
		chat = oai
		fmt.Fprintf(os.Stderr, "[chat] using provider: %s (model=%s)\n", prov.Name, model)
	} else if hasOldDeepSeek {
		chat = llm.NewDeepSeek(reg, cfg.DeepSeekKey, cfg.Temperature)
		fmt.Fprintf(os.Stderr, "[chat] using legacy DeepSeek key\n")
	} else if hasOldAnthropic {
		chat = llm.NewClaude(reg, cfg.AnthropicKey, cfg.Temperature)
		fmt.Fprintf(os.Stderr, "[chat] using legacy Anthropic key\n")
	} else {
		http.Error(w, "未配置可用模型(请先在设置中填入提供商 key 与模型)", http.StatusBadGateway)
		fmt.Fprintf(os.Stderr, "[chat] no key available, returning error\n")
		return
	}
	answer, err := chat.Chat(ctx, body.Question, body.History)
	if err != nil {
		http.Error(w, "对话服务不可用: "+err.Error(), http.StatusBadGateway)
		return
	}
	// 深度思考: 同步取思维链(修 defer 时序 —— 原实现 defer 在 writeJSON 后才执行, thinking 恒空)。
	if cfg.DeepThinking && oai != nil {
		thinking = oai.LastThinking()
	}
	writeJSON(w, map[string]any{"answer": answer, "thinking": thinking})
}
