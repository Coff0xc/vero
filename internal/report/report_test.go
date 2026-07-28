package report

import (
	"strings"
	"testing"

	"redcell/internal/core"
)

func TestMarkdownSortsAndCitesEvidence(t *testing.T) {
	g := core.NewAttackGraph()
	g.UpsertNode(&core.Node{ID: "service:x:80", Type: "service", Label: "http on x:80", State: core.StateConfirmed})
	g.UpsertNode(&core.Node{ID: "finding:info", Type: "finding", Label: "[info] Swagger API", State: core.StateConfirmed})
	g.UpsertNode(&core.Node{ID: "finding:sqli", Type: "finding",
		Label: "[critical] SQLi 登录绕过成功", State: core.StateConfirmed,
		Evidence: []core.Evidence{{Tool: "exploit_sqli", Excerpt: "token:abc123"}}})

	md := Markdown("http://target", g, 0, "2026-01-01")

	// critical 应排在 info 之前
	ci, ii := strings.Index(md, "SQLi"), strings.Index(md, "Swagger")
	if ci < 0 || ii < 0 || ci > ii {
		t.Fatalf("critical 应排在 info 前: sqli@%d swagger@%d", ci, ii)
	}
	if !strings.Contains(md, "token:abc123") {
		t.Fatal("报告应含逐字证据")
	}
	if !strings.Contains(md, "SQL 注入") {
		t.Fatal("报告应含 SQLi 修复建议")
	}
	if !strings.Contains(md, "全部发现逐字可溯源") {
		t.Fatal("0 违规应标注证据完整")
	}
}
