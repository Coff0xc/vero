// Package report —— 增强报告生成器：支持 PDF/HTML 导出 + CVSS 评分。
package report

import (
	"time"

	"github.com/Coff0xc/vero/internal/core"
)

// Report —— 完整渗透测试报告（可序列化为 JSON/PDF/HTML）。
type Report struct {
	Meta          ReportMeta          `json:"meta"`
	Executive     ExecutiveSummary    `json:"executive"`
	AttackSurface []Service           `json:"attack_surface"`
	Findings      []Finding           `json:"findings"`
	AttackGraph   *core.AttackGraph   `json:"attack_graph,omitempty"`
	Remediation   []Recommendation    `json:"remediation"`
	Timeline      *Timeline           `json:"timeline"`       // 攻击时间线
	AttackPath    *AttackPath         `json:"attack_path"`    // 攻击路径图
}

// ReportMeta —— 报告元数据。
type ReportMeta struct {
	Target      string    `json:"target"`
	GeneratedAt time.Time `json:"generated_at"`
	Engine      string    `json:"engine"`       // "REDCELL v1.0"
	CampaignID  string    `json:"campaign_id"`
	Duration    int       `json:"duration_sec"` // 战役耗时（秒）
}

// ExecutiveSummary —— 执行摘要（给 CISO/管理层看）。
type ExecutiveSummary struct {
	TotalServices  int     `json:"total_services"`
	TotalFindings  int     `json:"total_findings"`
	CriticalCount  int     `json:"critical_count"`
	HighCount      int     `json:"high_count"`
	MediumCount    int     `json:"medium_count"`
	LowCount       int     `json:"low_count"`
	RiskScore      float64 `json:"risk_score"`       // 0-10，综合风险评分
	EvidenceStatus string  `json:"evidence_status"`  // "✓ 全部可溯源" / "⚠ N 项证据违规"
}

// Service —— 发现的服务。
type Service struct {
	ID      string `json:"id"`       // "host:192.168.1.1:80"
	Label   string `json:"label"`    // "HTTP/1.1 nginx/1.18.0"
	Port    int    `json:"port"`
	Protocol string `json:"protocol"` // "http", "ssh", "smb"
}

// Finding —— 发现的漏洞/风险。
type Finding struct {
	ID          string     `json:"id"`
	Severity    string     `json:"severity"`     // "critical", "high", "medium", "low"
	Title       string     `json:"title"`
	Description string     `json:"description"`
	CVSS        CVSSScore  `json:"cvss"`
	Evidence    []Evidence `json:"evidence"`
	Remediation string     `json:"remediation"`
}

// CVSSScore —— CVSS v3.1 评分。
type CVSSScore struct {
	BaseScore  float64 `json:"base_score"`   // 0-10
	Vector     string  `json:"vector"`       // "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:N"
	Severity   string  `json:"severity"`     // "Critical", "High", "Medium", "Low"
}

// Evidence —— 证据片段。
type Evidence struct {
	Tool    string `json:"tool"`
	Excerpt string `json:"excerpt"`
}

// Recommendation —— 修复建议。
type Recommendation struct {
	Category    string `json:"category"`     // "SQL Injection", "Exposed API"
	Priority    string `json:"priority"`     // "Critical", "High"
	Description string `json:"description"`
	Steps       []string `json:"steps"`      // 具体修复步骤
}

// Format —— 报告导出格式。
type Format string

const (
	FormatMarkdown Format = "markdown"
	FormatHTML     Format = "html"
	FormatPDF      Format = "pdf"
	FormatJSON     Format = "json"
)
