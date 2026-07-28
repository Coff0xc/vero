package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"redcell/internal/tools"
)

// HITLThreshold —— >= L3(利用/提权)的动作必须人工批准。
const HITLThreshold = tools.LevelExploit

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
	g := NewAttackGraph()
	var history []HistoryItem
	var trace []string
	lastSig, stall, prevNodeCount := "", 0, 0

	for step := 0; step < budget; step++ {
		action := llm.Propose(goal, g, history)
		if action == nil {
			emit(Event{Kind: "done", Data: map[string]any{"reason": "no more actions"}})
			break
		}
		tool, ok := reg.Get(action.Tool)
		if !ok { // allowlist: 未注册工具直接拒(内核不执行图外指令)
			emit(Event{Kind: "tool", Data: map[string]any{"tool": action.Tool, "success": false, "stdout": "unknown tool: " + action.Tool}})
			if r, ok := llm.(Rejecter); ok {
				r.OnReject()
			}
			history = append(history, HistoryItem{Outcome: "rejected", Action: *action})
			continue
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
			history = append(history, HistoryItem{Outcome: "rejected", Action: *action})
			continue
		}

		res := tool.Run(action.Args)
		trace = append(trace, res.Stdout)
		emit(Event{Kind: "tool", Data: map[string]any{
			"tool": action.Tool, "level": level, "args": action.Args,
			"success": res.Success, "stdout": truncate(res.Stdout, 400), "stderr": truncate(res.Stderr, 200),
		}})
		if !res.Success {
			if r, ok := llm.(Rejecter); ok {
				r.OnReject() // 失败 -> 让规划器换路
			}
		}

		if res.Success {
			applyObservations(g, tool, action, res, emit)

			// claim: 声称默认 hypothesis(不可信), 独立验证才 confirm
			if action.Claim != "" {
				cid := "claim:" + action.Claim
				g.UpsertNode(&Node{ID: cid, Type: "finding", Label: action.Claim, State: StateHypothesis})
				emitGraph(emit, g, cid, "hypothesis")
			}
			// claim 即验证: 本动作 verifies 某 claim -> confirm
			if v := tools.ArgStr(action.Args, "verifies", ""); v != "" {
				cid := "claim:" + v
				if _, ok := g.Nodes[cid]; ok {
					_, _ = g.Confirm(cid, Evidence{Tool: action.Tool, Excerpt: truncate(strings.TrimSpace(res.Stdout), 200)})
					emitGraph(emit, g, cid, "confirm")
				}
			}
			// 规划产出: 规划步成功 -> 建 produces 类型节点(confirmed, 证据=工具输出)
			if action.Produces != "" {
				pt := tools.ArgStr(action.Args, "target", "?")
				pid := action.Produces + ":" + pt
				g.UpsertNode(&Node{ID: pid, Type: action.Produces, Label: pt})
				_, _ = g.Confirm(pid, Evidence{Tool: action.Tool, Excerpt: truncate(strings.TrimSpace(res.Stdout), 200)})
				emitGraph(emit, g, pid, "confirm")
			}
		}
		history = append(history, HistoryItem{Outcome: "done", Action: *action, Result: &res})

		// 停滞检测: 连续重复(tool+args)且攻击图无新增 -> 停止空转(防 LLM 分析瘫痪/无效循环)
		sig := action.Tool + "|" + fmt.Sprint(action.Args)
		if sig == lastSig && len(g.Nodes) == prevNodeCount {
			if stall++; stall >= 2 {
				emit(Event{Kind: "done", Data: map[string]any{"reason": "检测到重复无进展动作, 停止空转"}})
				break
			}
		} else {
			stall = 0
		}
		lastSig, prevNodeCount = sig, len(g.Nodes)
	}
	return g, trace
}

// applyObservations —— parser 驱动图: 结构化 observation -> 节点/边 + 证据绑定。
func applyObservations(g *AttackGraph, tool *tools.Tool, action *Action, res tools.ToolResult, emit EmitFunc) {
	if tool.Parse == nil {
		return
	}
	for _, o := range tool.Parse(res.Stdout, action.Args) {
		nid := o.Kind + ":" + o.Key
		g.UpsertNode(&Node{ID: nid, Type: o.Kind, Label: o.Label})
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
}

// emitGraph —— graph 事件携带节点 type + 最近证据(供前端点节点检视证据链)。
// key 为 "confirm" 或 "hypothesis", 前端据此着色。
func emitGraph(emit EmitFunc, g *AttackGraph, nid, key string) {
	n, ok := g.Nodes[nid]
	if !ok {
		return
	}
	ev := n.Evidence
	if len(ev) > 3 {
		ev = ev[len(ev)-3:]
	}
	emit(Event{Kind: "graph", Data: map[string]any{key: nid, "type": n.Type, "evidence": ev}})
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
