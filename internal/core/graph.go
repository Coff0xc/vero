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
	"time"
)

// 节点/边状态。
const (
	StateHypothesis = "hypothesis" // 假设(不可信, 默认)
	StateConfirmed  = "confirmed"  // 已证实(必须有 evidence)
	StateRefuted    = "refuted"    // 已证伪
)

// Evidence —— 一条证据: 来自哪个工具、逐字来源片段(证据回查的锚点)、捕获时间与置信度。
type Evidence struct {
	Tool       string  `json:"tool"`
	Excerpt    string  `json:"excerpt"`
	At         int64   `json:"at,omitempty"`         // 捕获时间(UTC unix), 供时间线/报告可信度(修假时间戳)
	Confidence float64 `json:"confidence,omitempty"` // 0..1, 该条证据置信度
}

// Node —— 攻击图节点(host/service/cred/finding/foothold)。
type Node struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	Label      string     `json:"label"`
	State      string     `json:"state"`
	Severity   string     `json:"severity,omitempty"`   // critical/high/medium/low/info(结构化, 不从 label 解析)
	Technique  string     `json:"technique,omitempty"`  // MITRE ATT&CK technique ID(如 T1190)
	Tactic     string     `json:"tactic,omitempty"`     // MITRE ATT&CK tactic(如 initial-access)
	Confidence float64    `json:"confidence,omitempty"` // 0..1, 节点证据的平均置信度
	CreatedAt  int64      `json:"created_at,omitempty"` // 首次创建(UTC unix), 修报告假时间戳
	UpdatedAt  int64      `json:"updated_at,omitempty"` // 最近更新
	Evidence   []Evidence `json:"evidence"`
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
	now := time.Now().Unix()
	if cur, ok := g.Nodes[n.ID]; ok {
		if len(n.Evidence) > 0 {
			// 修复 C5: 证据去重 - 检查 Tool + Excerpt 指纹
			for _, newEv := range n.Evidence {
				isDup := false
				for _, oldEv := range cur.Evidence {
					if oldEv.Tool == newEv.Tool && oldEv.Excerpt == newEv.Excerpt {
						isDup = true
						break
					}
				}
				if !isDup {
					cur.Evidence = append(cur.Evidence, newEv)
				}
			}
		}
		cur.UpdatedAt = now
		return cur
	}
	if n.State == "" {
		n.State = StateHypothesis
	}
	if n.CreatedAt == 0 {
		n.CreatedAt = now
	}
	n.UpdatedAt = now
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
	now := time.Now().Unix()
	if ev.At == 0 {
		ev.At = now
	}
	n.Evidence = append(n.Evidence, ev)
	n.State = StateConfirmed
	n.UpdatedAt = now
	if n.CreatedAt == 0 {
		n.CreatedAt = now
	}
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

// FindPath —— 沿 State==confirmed 的边 BFS 找从 startID 到任意 goalType 节点的最短路径。
// 返回节点 id 列表(含起止); 找不到返回 nil。
// 边与两端节点都必须 confirmed(evidence 约束): 只有坐实的推进才算攻击链,
// hypothesis 的边/节点不进主路径 —— 这是"抗幻觉"在主路径计算上的延续。
func (g *AttackGraph) FindPath(startID, goalType string) []string {
	start := g.Nodes[startID]
	if start == nil || start.State != StateConfirmed {
		return nil
	}
	// 起止同型: 单节点即路径。
	if start.Type == goalType {
		return []string{startID}
	}
	// 邻接表: 只收录 confirmed 边且两端节点均 confirmed。
	adj := map[string][]string{}
	for _, e := range g.Edges {
		if e.State != StateConfirmed {
			continue
		}
		s, d := g.Nodes[e.Src], g.Nodes[e.Dst]
		if s == nil || d == nil || s.State != StateConfirmed || d.State != StateConfirmed {
			continue
		}
		adj[e.Src] = append(adj[e.Src], e.Dst)
	}
	prev := map[string]string{}
	visited := map[string]bool{startID: true}
	queue := []string{startID}
	var goal string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if g.Nodes[cur].Type == goalType {
			goal = cur
			break
		}
		for _, nxt := range adj[cur] {
			if visited[nxt] {
				continue
			}
			visited[nxt] = true
			prev[nxt] = cur
			queue = append(queue, nxt)
		}
	}
	if goal == "" {
		return nil
	}
	path := []string{goal}
	for cur := prev[goal]; cur != ""; cur = prev[cur] {
		path = append([]string{cur}, path...)
		if cur == startID {
			break
		}
	}
	return path
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
