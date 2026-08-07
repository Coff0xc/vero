package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Coff0xc/vero/internal/tools"
)

// HITLThreshold —— >= L3(利用/提权)的动作必须人工批准。
const HITLThreshold = tools.LevelExploit

// reflectEvery —— 战役级反思间隔(BattleReflector): 每 N 步让决策器总结证伪/策略。
const reflectEvery = 4

// failStreakStall —— U4: 连续失败达到此阈值即判定停滞, 停止空转(不论 args 是否相同)。
const failStreakStall = 3

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
	lastSig, stall, prevNodeCount, failStreak := "", 0, 0, 0
	stop := false // U4: 停滞检测置位后终止整个循环(不只中断当前 plan)
	// U1: claim nid -> 创建该 claim 的动作目标 host。用于自动关联兜底:
	// 后续动作命中同一 host 且成功时, 视为对该假设的独立佐证(不依赖 claim 文本含 host, 也不依赖 LLM 填 verifies)。
	claimHost := map[string]string{}
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
			cont := runAction(ctx, g, &history, &trace, &lastSig, &stall, &prevNodeCount, &failStreak, &stop, claimHost, &phase,
				llm, reg, approve, emit, step, &acts[i])
			if !cont {
				break
			}
		}
		// U4: 停滞检测置位 -> 终止整个循环, 不再调 Propose 空转下一轮。
		if stop {
			break
		}
		// D29: 攻击链贯通(confirmed foothold/shell)即显式过渡到 done 并收尾,
		// 不再空耗预算等循环退出 —— 前端可立即展示"利用完成"终态。
		if phase != "done" && graphGoalReached(g) {
			phase = "done"
			emit(Event{Kind: "phase", Data: map[string]any{"phase": "done"}})
			emit(Event{Kind: "done", Data: map[string]any{"reason": "攻击链贯通(foothold/shell 已确认)"}})
			break
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
	lastSig *string, stall *int, prevNodeCount *int, failStreak *int, stop *bool, claimHost map[string]string, phase *string,
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
	// 参数规格校验(抄 PentAGI: 执行前按 schema 拦): 缺必填参数不执行,
	// 精确原因回喂决策器, 下轮按规格自纠 —— 不再瞎跑一个注定失败的命令。
	if msg := tools.ValidateArgs(tool, action.Args); msg != "" {
		emit(Event{Kind: "tool", Data: map[string]any{"tool": action.Tool, "success": false, "stdout": msg}})
		if rf, ok := llm.(Reflector); ok {
			rf.OnFailure(*action, msg)
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
	// 修复 L1/L2: 工具失败时检查是否可自动重试(网络超时等可恢复失败)
	if !res.Success {
		if rt, ok := llm.(Retrier); ok && rt.ShouldRetry(resultReason(res)) {
			retryArgs := rt.AdjustArgsForRetry(*action, resultReason(res))
			emit(Event{Kind: "tool", Data: map[string]any{
				"tool": action.Tool, "args": retryArgs, "success": false,
				"stdout": "自动重试中(调整参数)...", "stderr": resultReason(res),
			}})
			res = tool.Run(retryArgs)
			*trace = append(*trace, res.Stdout)
			emit(Event{Kind: "tool", Data: map[string]any{
				"tool": action.Tool, "level": level, "args": retryArgs,
				"success": res.Success, "stdout": truncate(res.Stdout, 400), "stderr": truncate(res.Stderr, 200),
			}})
		}
	}
	if !res.Success {
		*failStreak++ // U4: 连续失败计数(不论 args 是否相同), 供停滞检测兜底
		if r, ok := llm.(Rejecter); ok {
			r.OnReject() // 失败 -> 让规划器换路
		}
		if rf, ok := llm.(Reflector); ok { // 结构化反思: 失败原因(stderr/stdout 首行)回传决策器
			rf.OnFailure(*action, resultReason(res))
		}
	}
	if !res.Success {
		// U4: 连续失败兜底停滞检测必须在此 return 之前 —— 失败路径不会走到后面(成功路径)的
		// 停滞检测块。连续 failStreak 次失败(不论 args 是否相同)累积到阈值即停止空转,
		// 防 LLM 对同一不可达目标反复试探耗尽预算。
		if *failStreak >= failStreakStall {
			emit(Event{Kind: "done", Data: map[string]any{"reason": "连续失败无进展, 停止空转"}})
			*stop = true
			return false
		}
		return false // 计划模式: 失败即中断后续依赖步骤
	}
	*failStreak = 0
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
		// U1: 记录该 claim 的目标 host, 供后续动作自动关联(不依赖 claim 文本是否含 host 字面量)。
		if h := normHost(tools.ArgStr(action.Args, "target", "")); h != "" {
			if _, seen := claimHost[cid]; !seen {
				claimHost[cid] = h
			}
		}
		emitGraph(emit, g, cid, "hypothesis")
	}
	// claim 即验证: 本动作 verifies 某 claim -> confirm
	// 并补 rel="verifies" 的 confirmed 边(目标 host -> claim), 给 FindPath 提供真实边。
	// 修复 C3 兼容: verifies 可能来自结构体字段(DeepSeek/Claude)或 Args(LLM 随意放置)
	verifiesVal := action.Verifies
	if verifiesVal == "" {
		verifiesVal = tools.ArgStr(action.Args, "verifies", "")
	}
	if verifiesVal != "" {
		cid := "claim:" + verifiesVal
		if _, ok := g.Nodes[cid]; ok {
			_, _ = g.Confirm(cid, Evidence{Tool: action.Tool, Excerpt: truncate(strings.TrimSpace(res.Stdout), 200)})
			if host := "host:" + normHost(tools.ArgStr(action.Args, "target", "?")); g.Nodes[host] != nil && !g.HasEdge(host, "verifies", cid) {
				g.Edges = append(g.Edges, &Edge{
					Src: host, Rel: "verifies", Dst: cid, State: StateConfirmed,
					Evidence: []Evidence{{Tool: action.Tool, Excerpt: truncate(strings.TrimSpace(res.Stdout), 200)}},
				})
				emit(Event{Kind: "edge", Data: map[string]any{"src": host, "dst": cid, "rel": "verifies"}})
			}
			emitGraph(emit, g, cid, "confirm")
		}
	}
	// U1 兜底: LLM 不填 verifies 时, 用"独立后续动作命中同一 host"做自动关联。
	// 约束(防退化为自证):
	//   1) 只 confirm 由【之前】动作创建的 hypothesis claim, 排除本动作自己的 claim
	//      (self-assertion ≠ verification, 保持 TestUnverifiedClaimStaysHypothesis 不变式);
	//   2) 关联信号优先用创建 claim 时记录的 target host(claimHost), 回退到 claim 文本含 host,
	//      解决自然语言 claim(不含 host 字面量)永不触发的过严问题;
	//   3) 证据=本动作真实 stdout, 逐字回查约束不变。
	if verifiesVal == "" {
		host := normHost(tools.ArgStr(action.Args, "target", ""))
		selfClaim := ""
		if action.Claim != "" {
			selfClaim = "claim:" + action.Claim
		}
		if host != "" {
			for id, n := range g.Nodes {
				if n.Type != "finding" || n.State != StateHypothesis || !strings.HasPrefix(id, "claim:") {
					continue
				}
				if id == selfClaim {
					continue // 不自证: 本动作创建的 claim 不能由本动作确认
				}
				if claimHost[id] != host && !strings.Contains(n.Label, host) {
					continue // 目标不一致: 既非记录的 host, 文本也不含 host
				}
				_, _ = g.Confirm(id, Evidence{Tool: action.Tool, Excerpt: truncate(strings.TrimSpace(res.Stdout), 200)})
				emitGraph(emit, g, id, "confirm")
			}
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
	// 修复 C4: 用稳定的 JSON 序列化替代 fmt.Sprint(map)（map 遍历无序导致 sig 不稳定）
	// U4: 连续失败兜底已前移到失败分支(见上, 成功路径 failStreak 恒为 0), 此处只判 sig 重复。
	sig := action.Tool + "|" + stableArgsSig(action.Args)
	if sig == *lastSig && len(g.Nodes) == *prevNodeCount {
		if *stall++; *stall >= 2 {
			emit(Event{Kind: "done", Data: map[string]any{"reason": "检测到重复无进展动作, 停止空转"}})
			*stop = true
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
		// 修复 C1: 为 finding 类型建立关联边
		// U2 修复: Key 可能是 URL/自定义格式(http://host:tech:Via 等), 不能直接 Split(":")。
		// 用 URL 感知解析提取 host[:port]: 端口段非纯数字(如 tech)时只按 host 匹配。
		// U5 修复: 找不到 service(如 probe 模式)时回退建 host→exposes→finding 边, 保证图有拓扑。
		if o.Kind == "finding" {
			host, port := keyHostPort(o.Key)
			if host == "" {
				host, port = keyHostPort(tools.ArgStr(action.Args, "target", ""))
			}
			// 1) 优先匹配 service: host 精确前缀 + 端口(有则)命中
			var sourceService string
			if host != "" {
				for sid, sn := range g.Nodes {
					if sn.Type != "service" || !strings.HasPrefix(sid, "service:"+host+":") {
						continue
					}
					if port != "" && !strings.HasSuffix(sid, ":"+port) {
						continue
					}
					sourceService = sid
					if port != "" {
						break // host+port 全命中: 最佳匹配
					}
				}
			}
			// 2) 无 service 时: 确保 host 节点存在, 回退挂到 host 上(probe 模式兜底)
			src := sourceService
			if src == "" && host != "" {
				hid := "host:" + normHost(host)
				if g.Nodes[hid] == nil {
					g.UpsertNode(&Node{ID: hid, Type: "host", Label: normHost(host)})
					emitGraph(emit, g, hid, "confirm")
				}
				src = hid
			}
			// 3) 建边 service→exposes→finding 或 host→exposes→finding
			if src != "" && !g.HasEdge(src, "exposes", nid) {
				g.Edges = append(g.Edges, &Edge{
					Src: src, Rel: "exposes", Dst: nid, State: StateConfirmed,
					Evidence: []Evidence{{Tool: action.Tool, Excerpt: o.Excerpt}},
				})
				emit(Event{Kind: "edge", Data: map[string]any{"src": src, "dst": nid, "rel": "exposes"}})
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
		// D6: 带截断标记, 证据回查可识别"前缀截断"并兼容匹配。
		return string(r[:n]) + "…"
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
// 修复 C8: 当紧邻的前一阶段无 confirmed 节点时, 向前递归搜索更早的阶段(支持跨级跳跃)。
// 例如 service 直接产出 cred(跳过 web_shell), 应连 service→cred 而非因找不到 web_shell 而不建边。
func prevStageNode(g *AttackGraph, stageType, target string) string {
	prevStages := []string{}
	for i, t := range attackChainStages {
		if t == stageType {
			for j := i - 1; j >= 0; j-- {
				prevStages = append(prevStages, attackChainStages[j])
			}
			break
		}
	}
	if len(prevStages) == 0 {
		return ""
	}
	// 第一轮: 逐阶段找 host:port 匹配的 confirmed 节点(优先紧邻阶段)
	for _, prev := range prevStages {
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
	}
	// 第二轮: 回退到任意 confirmed 的前置阶段节点(按阶段优先级)
	for _, prev := range prevStages {
		for _, id := range g.Order {
			n := g.Nodes[id]
			if n.Type == prev && n.State == StateConfirmed {
				return id
			}
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

// graphGoalReached —— 攻击链是否已贯通: 存在 confirmed 的 foothold/shell 节点。
// D29: 贯通即视为战役主要目标达成, 提前广播 done 收尾。
func graphGoalReached(g *AttackGraph) bool {
	for _, id := range g.Order {
		n := g.Nodes[id]
		if (n.Type == "foothold" || n.Type == "shell") && n.State == StateConfirmed {
			return true
		}
	}
	return false
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

// stableArgsSig —— 生成稳定的参数签名（修复 C4: fmt.Sprint(map) 遍历无序导致 sig 不稳定）。
// 按 key 排序后 JSON 序列化，保证同一组参数始终产生相同的 sig。
// U4 补充: target 归一化(剥 scheme/尾斜杠), 避免 "http://host:8080" 与 "host:8080" 视为不同参数。
func stableArgsSig(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	norm := make(map[string]any, len(args))
	for k, v := range args {
		if k == "target" {
			if s, ok := v.(string); ok {
				norm[k] = normTarget(s)
				continue
			}
		}
		norm[k] = v
	}
	keys := make([]string, 0, len(norm))
	for k := range norm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// N3 修复: json.Marshal(map) 不保证顺序; 改为按排序后的 keys 手工构造 JSON,
	// 保证同组参数始终产生相同 sig(C4 的真正意图)。
	var buf strings.Builder
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(norm[k])
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.String()
}

// normTarget —— target 归一化: 去空白/尾斜杠, 剥 scheme, 用于停滞检测 sig 稳定。
func normTarget(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "/")
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	return s
}

// normHost —— 从可能带 scheme/路径/端口的字符串提取裸 host(用于 host 节点 ID 归一化)。
func normHost(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if strings.HasPrefix(s, "[") { // IPv6 [::1]:80
		if i := strings.Index(s, "]"); i >= 0 {
			return s[:i+1]
		}
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		if _, err := strconv.Atoi(s[i+1:]); err == nil {
			return s[:i]
		}
	}
	return s
}

// keyHostPort —— 从 observation Key(URL/裸 host:port/自定义格式)提取 host 与端口。
// 例: http://host:8080:tech:Via → host, 8080; host:80:x → host, 80; host:tech:Via → host, ""。
// 端口段非纯数字(如 tech)时视为类型标识, 返回空端口(仅按 host 匹配)。
func keyHostPort(key string) (string, string) {
	s := key
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ":")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	host := parts[0]
	if len(parts) >= 2 {
		if _, err := strconv.Atoi(parts[1]); err == nil {
			return host, parts[1]
		}
	}
	return host, ""
}
