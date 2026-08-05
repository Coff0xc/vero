// Package report —— 由攻击图生成渗透测试报告(Markdown 交付物)。
// 报告直接从证据链构建, 每个 finding 附逐字工具证据 —— 可交付、可复核、可追溯。
package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Coff0xc/vero/internal/core"
)

var sevRank = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "info": 4}

// sevOf —— 从 "[critical] xxx" 形式的 label 提取严重级。
func sevOf(label string) string {
	if strings.HasPrefix(label, "[") {
		if i := strings.Index(label, "]"); i > 0 {
			return strings.ToLower(label[1:i])
		}
	}
	return "info"
}

// sevOfNode —— finding 严重级: 优先读结构化 Node.Severity(parser 填的), 兼容旧 label 前缀。
func sevOfNode(n *core.Node) string {
	if n.Severity != "" {
		return n.Severity
	}
	return sevOf(n.Label)
}

func titleOf(label string) string {
	if strings.HasPrefix(label, "[") {
		if i := strings.Index(label, "]"); i > 0 {
			return strings.TrimSpace(label[i+1:])
		}
	}
	return label
}

// Markdown —— 生成渗透测试报告。target 目标, g 攻击图, violations 证据违规数, ts 时间戳字符串。
func Markdown(target string, g *core.AttackGraph, violations int, ts string) string {
	var findings, services []*core.Node
	for _, id := range g.Order {
		n := g.Nodes[id]
		switch n.Type {
		case "finding":
			if n.State == core.StateConfirmed {
				findings = append(findings, n)
			}
		case "service":
			services = append(services, n)
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		return sevRank[sevOfNode(findings[i])] < sevRank[sevOfNode(findings[j])]
	})

	crit := 0
	for _, f := range findings {
		if s := sevOfNode(f); s == "critical" || s == "high" {
			crit++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# 渗透测试报告 — %s\n\n", target)
	if ts != "" {
		fmt.Fprintf(&b, "> 生成时间: %s · 引擎: VERO\n\n", ts)
	}

	b.WriteString("## 执行摘要\n\n")
	fmt.Fprintf(&b, "- 目标: `%s`\n", target)
	fmt.Fprintf(&b, "- 发现服务: **%d**\n", len(services))
	fmt.Fprintf(&b, "- 确认发现: **%d**(高危 **%d**)\n", len(findings), crit)
	if violations == 0 {
		b.WriteString("- 证据完整性: **✓ 全部发现逐字可溯源**\n\n")
	} else {
		fmt.Fprintf(&b, "- 证据完整性: **⚠ %d 项证据违规(疑似幻觉)**\n\n", violations)
	}

	if len(services) > 0 {
		b.WriteString("## 攻击面\n\n| 服务 | 详情 |\n|------|------|\n")
		for _, s := range services {
			fmt.Fprintf(&b, "| `%s` | %s |\n", s.ID, s.Label)
		}
		b.WriteString("\n")
	}

	b.WriteString("## 发现明细(按严重级)\n\n")
	if len(findings) == 0 {
		b.WriteString("_未确认任何发现。_\n\n")
	}
	for i, f := range findings {
		fmt.Fprintf(&b, "### %d. [%s] %s\n\n", i+1, strings.ToUpper(sevOfNode(f)), titleOf(f.Label))
		if len(f.Evidence) > 0 {
			b.WriteString("**证据**(逐字来自工具输出):\n\n")
			for _, ev := range f.Evidence {
				fmt.Fprintf(&b, "```\n[%s] %s\n```\n\n", ev.Tool, ev.Excerpt)
			}
		}
	}

	b.WriteString("## 修复建议\n\n")
	b.WriteString(recommendations(findings))

	return b.String()
}

func recommendations(findings []*core.Node) string {
	seen := map[string]bool{}
	var b strings.Builder
	for _, f := range findings {
		var rec string
		switch {
		case strings.Contains(f.Label, "SQLi") || strings.Contains(f.Label, "SQL"):
			rec = "- **SQL 注入**: 改用参数化查询/ORM, 登录接口做严格输入校验, 部署 WAF。"
		case strings.Contains(f.Label, "Swagger") || strings.Contains(f.Label, "API"):
			rec = "- **API 文档暴露**: 生产环境关闭 Swagger/api-docs 公开访问或加认证。"
		case strings.Contains(f.Label, "Prometheus") || strings.Contains(f.Label, "Metrics"):
			rec = "- **监控端点暴露**: 限制 /metrics 为内网/认证访问, 防信息泄露。"
		case strings.Contains(f.Label, "Security Headers"):
			rec = "- **缺失安全头**: 补齐 CSP / HSTS / X-Frame-Options 等安全响应头。"
		default:
			continue
		}
		if !seen[rec] {
			seen[rec] = true
			b.WriteString(rec + "\n")
		}
	}
	if b.Len() == 0 {
		return "- 未发现需紧急修复项; 建议定期复测。\n"
	}
	return b.String()
}
