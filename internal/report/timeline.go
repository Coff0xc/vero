// 报告增强：时间线和攻击路径
package report

import (
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/core"
)

// Timeline —— 攻击时间线事件
type Timeline struct {
	Events []TimelineEvent `json:"events"`
}

// TimelineEvent —— 单个时间线事件
type TimelineEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Phase       string    `json:"phase"`       // "reconnaissance", "exploitation", "post-exploitation"
	Action      string    `json:"action"`      // "discovered service", "exploited vulnerability"
	Description string    `json:"description"`
	NodeID      string    `json:"node_id"`
	Critical    bool      `json:"critical"`
}

// AttackPath —— 攻击路径图
type AttackPath struct {
	Nodes []PathNode `json:"nodes"`
	Edges []PathEdge `json:"edges"`
}

// PathNode —— 路径节点
type PathNode struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`      // "host", "service", "credential", "finding"
	Label    string   `json:"label"`
	Critical bool     `json:"critical"`
	Evidence []string `json:"evidence"`
}

// PathEdge —— 路径边
type PathEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Label  string `json:"label"`       // "exploited", "discovered", "extracted"
	Method string `json:"method"`      // "sqli", "smb_enum", "kerberoast"
}

// GenerateTimeline —— 从攻击图生成时间线
func GenerateTimeline(g *core.AttackGraph) *Timeline {
	timeline := &Timeline{Events: []TimelineEvent{}}

	// 遍历所有节点，按创建顺序生成事件
	for _, id := range g.Order {
		node := g.Nodes[id]
		if node.State != core.StateConfirmed {
			continue
		}

		// 修假时间戳: 用节点真实创建时刻(UTC), 兜底旧数据才用 now。
		ts := time.Now()
		if node.CreatedAt > 0 {
			ts = time.Unix(node.CreatedAt, 0)
		}
		event := TimelineEvent{
			Timestamp:   ts,
			NodeID:      id,
			Description: node.Label,
		}

		// 根据节点类型确定阶段
		switch node.Type {
		case "host":
			event.Phase = "reconnaissance"
			event.Action = "discovered host"
		case "service":
			event.Phase = "reconnaissance"
			event.Action = "discovered service"
		case "finding":
			event.Phase = "exploitation"
			event.Action = "identified vulnerability"
			event.Critical = isCriticalFinding(node.Label)
		case "cred":
			event.Phase = "post-exploitation"
			event.Action = "extracted credentials"
			event.Critical = true
		case "foothold":
			event.Phase = "post-exploitation"
			event.Action = "established foothold"
			event.Critical = true
		default:
			event.Phase = "other"
			event.Action = "unknown"
		}

		timeline.Events = append(timeline.Events, event)
	}

	return timeline
}

// GenerateAttackPath —— 从攻击图生成可视化路径
// 修启发式推断: 直接用攻击图里 State==confirmed 的真实边(host-runs-service 等),
// 不再 O(n^2) 遍历所有节点对靠类型猜边。
func GenerateAttackPath(g *core.AttackGraph) *AttackPath {
	path := &AttackPath{Nodes: []PathNode{}, Edges: []PathEdge{}}

	for id, node := range g.Nodes {
		if node.State != core.StateConfirmed {
			continue
		}
		path.Nodes = append(path.Nodes, PathNode{
			ID:       id,
			Type:     node.Type,
			Label:    node.Label,
			Critical: isCriticalNode(node),
			Evidence: extractEvidenceKeys(node),
		})
	}

	for _, e := range g.Edges {
		if e.State != core.StateConfirmed {
			continue
		}
		from, ok1 := g.Nodes[e.Src]
		to, ok2 := g.Nodes[e.Dst]
		if !ok1 || !ok2 || from.State != core.StateConfirmed || to.State != core.StateConfirmed {
			continue
		}
		path.Edges = append(path.Edges, PathEdge{
			From:   e.Src,
			To:     e.Dst,
			Label:  e.Rel,
			Method: edgeEvidenceTool(e),
		})
	}
	return path
}

// edgeEvidenceTool —— 从边的证据里取第一个工具名作为利用方法(对应 edge.Method)。
func edgeEvidenceTool(e *core.Edge) string {
	if len(e.Evidence) > 0 && e.Evidence[0].Tool != "" {
		return e.Evidence[0].Tool
	}
	return "unknown"
}

// isCriticalFinding —— 判断是否为关键发现
func isCriticalFinding(label string) bool {
	criticalKeywords := []string{"critical", "exploit", "rce", "sqli", "admin"}
	for _, kw := range criticalKeywords {
		if contains(label, kw) {
			return true
		}
	}
	return false
}

// isCriticalNode —— 判断是否为关键节点
func isCriticalNode(node *core.Node) bool {
	if node.Type == "finding" || node.Type == "cred" || node.Type == "foothold" {
		return isCriticalFinding(node.Label)
	}
	return false
}

// extractEvidenceKeys —— 提取证据关键字
func extractEvidenceKeys(node *core.Node) []string {
	keys := []string{}
	// Evidence 是切片，遍历所有证据
	for _, ev := range node.Evidence {
		if ev.Excerpt != "" {
			excerpt := ev.Excerpt
			if len(excerpt) > 50 {
				excerpt = excerpt[:50] + "..."
			}
			keys = append(keys, excerpt)
		}
	}
	return keys
}

// inferEdgeLabel —— 推断边的标签
func inferEdgeLabel(fromType, toType string) string {
	switch {
	case fromType == "host" && toType == "service":
		return "runs"
	case fromType == "service" && toType == "finding":
		return "has vulnerability"
	case fromType == "finding" && toType == "cred":
		return "extracted"
	case fromType == "cred" && toType == "foothold":
		return "established"
	default:
		return "leads to"
	}
}

// inferMethod —— 推断利用方法
func inferMethod(node *core.Node) string {
	// Evidence 是切片，取第一个工具
	if len(node.Evidence) > 0 && node.Evidence[0].Tool != "" {
		return node.Evidence[0].Tool
	}
	return "unknown"
}

// shouldIncludeEdge —— 判断是否应包含这条边
func shouldIncludeEdge(from, to *core.Node) bool {
	// 简化：只包含有逻辑关系的边
	// 例如：host -> service, service -> finding
	if from.Type == "host" && to.Type == "service" {
		return true
	}
	if from.Type == "service" && to.Type == "finding" {
		return true
	}
	if from.Type == "finding" && (to.Type == "cred" || to.Type == "foothold") {
		return true
	}
	return false
}

// contains —— 检查字符串包含(不区分大小写)。
// 修复原实现: 只判断两串非空即恒真, 导致所有 finding 全被标 Critical。
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
