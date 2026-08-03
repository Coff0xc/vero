package core

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Coff0xc/vero/internal/tools"
)

// HITLThreshold —— >= L3(利用/提权)的动作必须人工批准。
const HITLThreshold = tools.LevelExploit

// reflectEvery —— 战役级反思间隔(BattleReflector): 每 N 步让决策器总结证伪/策略。
const reflectEvery = 4

// Approve —— HITL 审批回调: 返回是否放行(CLI 与 Web 各有实现)。
type Approve func(a Action, level int) bool

// EmitFunc —— 事件广播回调(默认打印; server 换成 SSE 广播)。
type EmitFunc func(e Event)

// AutoApprove —— 全部放行(离线自检 / eval 用)。
func AutoApprove(a Action, level int) bool { return true }

// DiscardEmit —— 丢弃所有事件(自检用)。
func DiscardEmit(e Event) {}

// CLIApprove —— 命令行 HITL: 逐条问 y/N。
func CLIApprove(a Action, level int) bool {
	fmt.Printf("[HITL] L%d 动作 %s%v | 理由: %s\n批准? [y/N] ", level, a.Tool, a.Args, a.Rationale)
	sc := bufio.NewScanner(os.Stdin)
	if sc.Scan() {
		return strings.EqualFold(strings.TrimSpace(sc.Text()), "y")
	}
	return false
}

// RunAgent —— 主循环(对应 Python run_agent)。
// 返回攻击图与工具输出 trace(所有工具原始 stdout, 是证据逐字回查的真相源)。
func RunAgent(goal string, llm LLM, reg *tools.Registry, approve Approve, emit EmitFunc, budget int) (*AttackGraph, []string) {
	return RunAgentCtx(context.Background(), goal, llm, reg, approve, emit, budget)
}

// emitNoActions —— 决策器判定结束时广播: 若决策器记录了失败原因(如 API 错误/无效模型名),
// 先广播 error 事件(前端红字展示), 再广播 done。
func emitNoActions(emit EmitFunc, llm LLM) {
	if er, ok := llm.(ErrorReporter); ok && er.LastError() != "" {
		emit(Event{Kind: "error", Data: map[string]any{"msg": er.LastError()}})
	}
	emit(Event{Kind: "done", Data: map[string]any{"reason": "no more actions"}})
}

// RunAgentCtx —— 带取消上下文的主循环: 每步检查 ctx, 战役可被操作员中途停止(不再永久卡死)。
func RunAgentCtx(ctx context.Context, goal string, llm LLM, reg *tools.Registry, approve Approve, emit EmitFunc, budget int) (*AttackGraph, []string) {
	g := NewAttackGraph()
	var history []HistoryItem
	var trace []string
	lastSig, stall, prevNodeCount := "", 0, 0
	// 显式阶段状态机: init→recon→scan→exploit→done。按成功工具杀伤级推进, 每推进一次广播 phase 事件。
	phase := "init"
	emit(Event{Kind: "phase", Data: map[string]any{"phase": "init"}})

	for step := 0; step < budget; step++ {
		if ctx.Err() != nil { // 取消: 立即停, 保留已确认证据
			emit(Event{Kind: "done", Data: map[string]any{"reason": "战役已被操作员取消"}})
			break
		}
		// 战役级反思: 每 reflectEvery 步一次(有实质动作后), 决策器总结证伪/策略,
		// 自行缓存并注入下轮 prompt —— 把探索式搜索从“盲目试”变为“基于证伪收敛”。
		if step > 0 && step%reflectEvery == 0 {
			if br, ok := llm.(BattleReflector); ok {
				if txt := br.Reflect(goal, g, history); txt != "" {
					emit(Event{Kind: "reflect", Data: map[string]any{"text": txt}})
				}
			}
		}
		// 决策: 支持多步规划的 LLM 一次给一段计划(链式推理);
		// 老式 LLM 仍按单步 Propose。两者都返回 nil = 决策器认为该结束了。
		var acts []Action
		if pl, ok := llm.(Planner); ok {
			p := pl.ProposePlan(goal, g, history)
			if p == nil || len(p.Actions) == 0 {
				emitNoActions(emit, llm)
				break
			}
			if len(p.Actions) > 1 {
				emit(Event{Kind: "plan", Data: map[string]any{
					"count": len(p.Actions), "rationale": p.Rationale,
				}})
			}
			acts = p.Actions
		} else {
			a := llm.Propose(goal, g, history)
			if a == nil {
				emitNoActions(emit, llm)
				break
			}
			acts = []Action{*a}
		}
		// 深度思考: 决策器带思维链时广播 thinking 事件(前端折叠展示思考过程)。
		if tr, ok := llm.(ThinkingReporter); ok {
			if t := tr.LastThinking(); t != "" {
				emit(Event{Kind: "thinking", Data: map[string]any{"text": t}})
			}
		}
		// 计划按序执行: 每步独立 HITL/证据/停滞检测; 某步失败或被拒立即中断后续步
		// (计划是依赖链, 前提没立住后面跑了也是白跑)。
		for i := range acts {
			if ctx.Err() != nil {
				break
			}
			cont := runAction(ctx, g, &history, &trace, &lastSig, &stall, &prevNodeCount, &phase,
				llm, reg, approve, emit, step, &acts[i])
			if !cont {
				break
			}
		}
	}
	// 战役结束(预算耗尽/无动作/取消/停滞): 广播 done 阶段, 前端据此收尾。
	if phase != "done" {
		phase = "done"
		emit(Event{Kind: "phase", Data: map[string]any{"phase": "done"}})
	}
	return g, trace
}

// runAction —— 执行单个动作(注册校验/HITL 门控/执行/图更新/停滞检测), 返回是否继续后续动作。
// false = 该步未成功执行(未知工具/被拒/失败), 计划模式下中断剩余步骤。
func runAction(ctx context.Context, g *AttackGraph, history *[]HistoryItem, trace *[]string,
	lastSig *string, stall *int, prevNodeCount *int, phase *string,
	llm LLM, reg *tools.Registry, approve Approve, emit EmitFunc, step int, action *Action) bool {

	tool, ok := reg.Get(action.Tool)
	if !ok { // allowlist: 未注册工具直接拒(内核不执行图外指令)
		emit(Event{Kind: "tool", Data: map[string]any{"tool": action.Tool, "success": false, "stdout": "unknown tool: " + action.Tool}})
		if r, ok := llm.(Rejecter); ok {
			r.OnReject()
		}
		if rf, ok := llm.(Reflector); ok { // 结构化反思: 把具体失败原因回传决策器
			rf.OnFailure(*action, "unknown tool: "+action.Tool)
		}
		*history = append(*history, HistoryItem{Outcome: "rejected", Action: *action})
		return false
	}
	level := tool.Level
	emit(Event{Kind: "step", Data: map[string]any{
		"step": step, "tool": action.Tool, "args": action.Args, "level": level, "why": action.Rationale,
	}})

	// HITL 门控: L3+ 动作必须人工批准
	if level >= HITLThreshold && !approve(*action, level) {
		emit(Event{Kind: "hitl", Data: map[string]any{"action": action.Tool, "approved": false}})
		if r, ok := llm.(Rejecter); ok {
			r.OnReject() // 被拒 -> 让规划器换路, 不空转
		}
		if rf, ok := llm.(Reflector); ok { // 结构化反思: HITL 拒绝原因回传决策器
			rf.OnFailure(*action, "未通过人工审批(HITL 拒绝)")
		}
		*history = append(*history, HistoryItem{Outcome: "rejected", Action: *action})
		return false
	}

	res := tool.Run(action.Args)
	*trace = append(*trace, res.Stdout)
	emit(Event{Kind: "tool", Data: map[string]any{
		"tool": action.Tool, "level": level, "args": action.Args,
		"success": res.Success, "stdout": truncate(res.Stdout, 400), "stderr": truncate(res.Stderr, 200),
	}})
	if !res.Success {
		if r, ok := llm.(Rejecter); ok {
			r.OnReject() // 失败 -> 让规划器换路
		}
		if rf, ok := llm.(Reflector); ok { // 结构化反思: 失败原因(stderr/stdout 首行)回传决策器
			rf.OnFailure(*action, resultReason(res))
		}
	}
	if !res.Success {
		return false // 计划模式: 失败即中断后续依赖步骤
	}
	// 成功 -> 按工具杀伤级推进阶段状态机(init→recon→scan→exploit)。
	advancePhase(emit, phase, level)
	// 固定 parser 结构化; 返回产出条数, 供 LLM-as-observer 兜底判断。
	n := applyObservations(g, tool, action, res, emit)
	// LLM-as-observer(黑盒智能渗透核心): 固定 parser 无产出时, 让决策器用语言理解力
	// 从原始 stdout 提取观察 —— 证据约束不变(Excerpt 逐字回查, 防 LLM 编造)。
	if n == 0 {
		if ob, ok := llm.(Observer); ok {
			for _, o := range ob.Observe(action.Tool, action.Args, res.Stdout) {
				nid := o.Kind + ":" + o.Key
				g.UpsertNode(&Node{ID: nid, Type: o.Kind, Label: o.Label, Severity: o.Severity, Technique: o.Technique, Tactic: o.Tactic})
				_, _ = g.Confirm(nid, Evidence{Tool: action.Tool, Excerpt: o.Excerpt})
				emitGraph(emit, g, nid, "confirm")
			}
		}
	}

	// claim: 声称默认 hypothesis(不可信), 独立验证才 confirm
	if action.Claim != "" {
		cid := "claim:" + action.Claim
		g.UpsertNode(&Node{ID: cid, Type: "finding", Label: action.Claim, State: StateHypothesis})
		emitGraph(emit, g, cid, "hypothesis")
	}
	// claim 即验证: 本动作 verifies 某 claim -> confirm
	// 并补 rel="verifies" 的 confirmed 边(目标 host -> claim), 给 FindPath 提供真实边。
	if v := tools.ArgStr(action.Args, "verifies", ""); v != "" {
		cid := "claim:" + v
		if _, ok := g.Nodes[cid]; ok {
			_, _ = g.Confirm(cid, Evidence{Tool: action.Tool, Excerpt: truncate(strings.TrimSpace(res.Stdout), 200)})
			if host := "host:" + tools.ArgStr(action.Args, "target", "?"); g.Nodes[host] != nil && !g.HasEdge(host, "verifies", cid) {
				g.Edges = append(g.Edges, &Edge{
					Src: host, Rel: "verifies", Dst: cid, State: StateConfirmed,
					Evidence: []Evidence{{Tool: action.Tool, Excerpt: truncate(strings.TrimSpace(res.Stdout), 200)}},
				})
				emit(Event{Kind: "edge", Data: map[string]any{"src": host, "dst": cid, "rel": "verifies"}})
			}
			emitGraph(emit, g, cid, "confirm")
		}
	}
	// 规划产出: 规划步成功 -> 建 produces 类型节点(confirmed, 证据=工具输出)
	// 并补 rel="produces" 的 confirmed 边(上一阶段节点 -> 产物), 给 FindPath 提供真实攻击链边。
	// 动作未显式标注 produces 时, 用工具级默认产出(Tool.Produces)兜底 ——
	// 攻击链不依赖 LLM 自觉填字段(黑盒自主模式鲁棒性)。
	pType := action.Produces
	if pType == "" {
		pType = tool.Produces
	}
	if pType != "" {
		pt := tools.ArgStr(action.Args, "target", "?")
		pid := pType + ":" + pt
		g.UpsertNode(&Node{ID: pid, Type: pType, Label: pt})
		_, _ = g.Confirm(pid, Evidence{Tool: action.Tool, Excerpt: truncate(strings.TrimSpace(res.Stdout), 200)})
		if prev := prevStageNode(g, pType, pt); prev != "" && !g.HasEdge(prev, "produces", pid) {
			g.Edges = append(g.Edges, &Edge{
				Src: prev, Rel: "produces", Dst: pid, State: StateConfirmed,
				Evidence: []Evidence{{Tool: action.Tool, Excerpt: truncate(strings.TrimSpace(res.Stdout), 200)}},
			})
			emit(Event{Kind: "edge", Data: map[string]any{"src": prev, "dst": pid, "rel": "produces"}})
		}
		emitGraph(emit, g, pid, "confirm")
	}
	*history = append(*history, HistoryItem{Outcome: "done", Action: *action, Result: &res})

	// 停滞检测: 连续重复(tool+args)且攻击图无新增 -> 停止空转(防 LLM 分析瘫痪/无效循环)
	sig := action.Tool + "|" + fmt.Sprint(action.Args)
	if sig == *lastSig && len(g.Nodes) == *prevNodeCount {
		if *stall++; *stall >= 2 {
			emit(Event{Kind: "done", Data: map[string]any{"reason": "检测到重复无进展动作, 停止空转"}})
			return false
		}
	} else {
		*stall = 0
	}
	*lastSig, *prevNodeCount = sig, len(g.Nodes)
	return true
}

// applyObservations —— parser 驱动图: 结构化 observation -> 节点/边 + 证据绑定。
// 返回产出条数(供 LLM-as-observer 兜底判断: 0 条时交决策器深度理解)。
func applyObservations(g *AttackGraph, tool *tools.Tool, action *Action, res tools.ToolResult, emit EmitFunc) int {
	if tool.Parse == nil {
		return 0
	}
	parsed := tool.Parse(res.Stdout, action.Args)
	for _, o := range parsed {
		nid := o.Kind + ":" + o.Key
		// TTP/severity 走结构化字段(parser 填的), 不从 label 解析; 报告直接读 Node.Severity。
		g.UpsertNode(&Node{ID: nid, Type: o.Kind, Label: o.Label, Severity: o.Severity, Technique: o.Technique, Tactic: o.Tactic})
		_, _ = g.Confirm(nid, Evidence{Tool: action.Tool, Excerpt: o.Excerpt})
		if o.Kind == "service" {
			host := strings.SplitN(o.Key, ":", 2)[0]
			hid := "host:" + host
			g.UpsertNode(&Node{ID: hid, Type: "host", Label: host})
			if !g.HasEdge(hid, "runs", nid) { // 边去重: 工具多次调用不重复建边
				g.Edges = append(g.Edges, &Edge{
					Src: hid, Rel: "runs", Dst: nid, State: StateConfirmed,
					Evidence: []Evidence{{Tool: action.Tool, Excerpt: o.Excerpt}},
				})
				emit(Event{Kind: "edge", Data: map[string]any{"src": hid, "dst": nid, "rel": "runs"}})
			}
		}
		emitGraph(emit, g, nid, "confirm")
	}
	return len(parsed)
}

// emitGraph —— graph 事件携带节点 type + 最近证据(供前端点节点检视证据链)。
// key 为 "confirm" 或 "hypothesis", 前端据此着色。
func emitGraph(emit EmitFunc, g *AttackGraph, nid, key string) {
	n, ok := g.Nodes[nid]
	if !ok {
		return
	}
	// 完整证据链 + 状态 + 结构化字段(severity/technique/tactic/confidence): 不再截断,
	// 前端按 state 只升不降合并。
	emit(Event{Kind: "graph", Data: map[string]any{
		key: nid, "type": n.Type, "state": n.State, "evidence": n.Evidence,
		"severity": n.Severity, "technique": n.Technique, "tactic": n.Tactic, "confidence": n.Confidence,
	}})
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// resultReason —— 工具执行失败的原因摘要(供 Reflector.OnFailure 做结构化反思):
// 优先 stderr 首行, 否则 stdout 首行。
func resultReason(res tools.ToolResult) string {
	if first := firstLine(res.Stderr); first != "" {
		return first
	}
	if first := firstLine(res.Stdout); first != "" {
		return first
	}
	return "工具失败(无输出)"
}

// firstLine —— 取字符串首行(去首尾空白)。
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// attackChainStages —— 攻击链阶段顺序: produces 节点按此连成推进边(service→web_shell→cred→foothold→shell)。
// 与前端 CHAIN 对齐; 真实规划链(planner/LLM produces)都走这条链。
var attackChainStages = []string{"service", "web_shell", "cred", "foothold", "shell"}

// prevStageNode —— produces 产物类型 stageType 的上一阶段已 confirmed 节点。
// 优先选 label 含 target 的节点(如产物 web_shell:http://host:3000 应挂在 service:host:3000 上,
// 而非先扫到的 445), 回退按插入序取首个。
// 用于把 produces 节点连成攻击链边(如 service -produces-> web_shell -produces-> cred -produces-> foothold)。
func prevStageNode(g *AttackGraph, stageType, target string) string {
	prev := ""
	for i, t := range attackChainStages {
		if t == stageType {
			if i > 0 {
				prev = attackChainStages[i-1]
			}
			break
		}
	}
	if prev == "" {
		return ""
	}
	// 第一轮: 前置阶段中 host:port 与产物 target 一致的(如产物 web_shell:http://host:3000
	// 挂在 service:localhost:3000 上, 而非先扫到的 445)。host:port 规范化后比较。
	if target != "" {
		hp := hostPortOf(target)
		if hp != "" {
			for _, id := range g.Order {
				n := g.Nodes[id]
				if n.Type == prev && n.State == StateConfirmed && strings.Contains(n.Label, hp) {
					return id
				}
			}
		}
	}
	// 回退: 任意前置阶段 confirmed 节点(插入序首个)。
	for _, id := range g.Order {
		n := g.Nodes[id]
		if n.Type == prev && n.State == StateConfirmed {
			return id
		}
	}
	return ""
}

// hostPortOf —— 从 URL/主机串里提取 host:port(去 scheme/路径; 无端口保留 host)。
func hostPortOf(s string) string {
	hp := s
	if i := strings.Index(hp, "://"); i >= 0 {
		hp = hp[i+3:]
	}
	if j := strings.IndexAny(hp, "/?"); j >= 0 {
		hp = hp[:j]
	}
	return hp
}

// advancePhase —— 显式阶段状态机: 按成功工具杀伤级推进 init→recon→scan→exploit。
// L0-1→recon, L2→scan, L3+→exploit; 只前进不回退(exploit 达成后不再降级)。
// 每推进一次广播 phase 事件, 供前端展示当前攻击阶段。
func advancePhase(emit EmitFunc, phase *string, level int) {
	target := "recon"
	switch {
	case level >= tools.LevelExploit: // L3+
		target = "exploit"
	case level >= tools.LevelCred: // L2
		target = "scan"
	default: // L0-L1
		target = "recon"
	}
	if *phase == "exploit" {
		return // 已到 exploit, 不回退
	}
	if *phase != target {
		*phase = target
		emit(Event{Kind: "phase", Data: map[string]any{"phase": target}})
	}
}
