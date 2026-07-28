// Package core —— 自主红队智能体内核: 攻击图 + 主循环 + LLM/HITL 契约。
//
// 核心闭环: 目标 -> LLM 提议 Action -> (危险则 HITL 拦截) -> 执行 -> parser 结构化
//           -> 更新攻击图(证据约束) -> 循环。
// 可靠性三件套(把抗幻觉做进数据结构层, 而非靠 prompt 兜底):
//   1. 结构化       —— parser 把工具输出变成带逐字来源的 Observation。
//   2. 证据逐字回查 —— confirmed 节点的证据必须能在工具原文里逐字找到(VerifyEvidence)。
//   3. claim 即验证 —— LLM 的声称默认 hypothesis, 独立验证才升 confirmed。
package core

import (
	"fmt"
	"strings"
)

// 节点/边状态。
const (
	StateHypothesis = "hypothesis" // 假设(不可信, 默认)
	StateConfirmed  = "confirmed"  // 已证实(必须有 evidence)
	StateRefuted    = "refuted"    // 已证伪
)

// Evidence —— 一条证据: 来自哪个工具、逐字来源片段(证据回查的锚点)。
type Evidence struct {
	Tool    string `json:"tool"`
	Excerpt string `json:"excerpt"`
}

// Node —— 攻击图节点(host/service/cred/finding/foothold)。
type Node struct {
	ID       string     `json:"id"`
	Type     string     `json:"type"`
	Label    string     `json:"label"`
	State    string     `json:"state"`
	Evidence []Evidence `json:"evidence"`
}

// Edge —— 攻击图边。
type Edge struct {
	Src      string     `json:"src"`
	Rel      string     `json:"rel"`
	Dst      string     `json:"dst"`
	State    string     `json:"state"`
	Evidence []Evidence `json:"evidence"`
}

// AttackGraph —— 攻击图(内存态; 证据约束是核心)。Order 保插入序, 让 snapshot 稳定可复现。
type AttackGraph struct {
	Nodes map[string]*Node `json:"nodes"`
	Order []string         `json:"order"`
	Edges []*Edge          `json:"edges"`
}

func NewAttackGraph() *AttackGraph {
	return &AttackGraph{Nodes: map[string]*Node{}}
}

// UpsertNode 插入或合并节点(已存在则并入证据, 不覆盖状态)。
func (g *AttackGraph) UpsertNode(n *Node) *Node {
	if cur, ok := g.Nodes[n.ID]; ok {
		if len(n.Evidence) > 0 {
			cur.Evidence = append(cur.Evidence, n.Evidence...)
		}
		return cur
	}
	if n.State == "" {
		n.State = StateHypothesis
	}
	g.Nodes[n.ID] = n
	g.Order = append(g.Order, n.ID)
	return n
}

// Confirm 证据约束: 无 evidence 不得 confirmed —— 从数据结构层抑制幻觉(对应 Python confirm)。
func (g *AttackGraph) Confirm(id string, ev Evidence) (*Node, error) {
	n, ok := g.Nodes[id]
	if !ok {
		return nil, fmt.Errorf("confirm(%s): 节点不存在", id)
	}
	if ev == (Evidence{}) {
		return nil, fmt.Errorf("confirm(%s): 缺 evidence, 拒绝写入", id)
	}
	n.Evidence = append(n.Evidence, ev)
	n.State = StateConfirmed
	return n, nil
}

// HasEdge —— 是否已有同 src-rel-dst 的边(去重用)。
func (g *AttackGraph) HasEdge(src, rel, dst string) bool {
	for _, e := range g.Edges {
		if e.Src == src && e.Rel == rel && e.Dst == dst {
			return true
		}
	}
	return false
}

// Snapshot —— 喂给 LLM 的紧凑状态视图(每轮现组, 不塞全历史)。
func (g *AttackGraph) Snapshot() string {
	var b strings.Builder
	b.WriteString("NODES:\n")
	if len(g.Order) == 0 {
		b.WriteString("  (空)\n")
	}
	for _, id := range g.Order {
		n := g.Nodes[id]
		fmt.Fprintf(&b, "  %s (%s,%s): %s\n", n.ID, n.Type, n.State, n.Label)
	}
	b.WriteString("EDGES:")
	if len(g.Edges) == 0 {
		b.WriteString("\n  (空)")
	}
	for _, e := range g.Edges {
		fmt.Fprintf(&b, "\n  %s -%s-> %s", e.Src, e.Rel, e.Dst)
	}
	return b.String()
}

// VerifyEvidence —— 证据逐字回查: confirmed 节点的每条 evidence.Excerpt 必须逐字出现在某次工具输出里。
// 抓 "LLM 声称拿到但工具输出里根本没有" 的幻觉证据(对应 Python verify_evidence)。
func VerifyEvidence(g *AttackGraph, trace []string) []string {
	blob := strings.Join(trace, "\n")
	var bad []string
	for _, id := range g.Order {
		n := g.Nodes[id]
		if n.State != StateConfirmed {
			continue
		}
		for _, ev := range n.Evidence {
			if ev.Excerpt != "" && !strings.Contains(blob, ev.Excerpt) {
				bad = append(bad, fmt.Sprintf("%s: 证据 %q 未在工具输出中逐字找到 (疑似幻觉)", n.ID, ev.Excerpt))
			}
		}
	}
	return bad
}
