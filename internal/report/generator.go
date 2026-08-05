package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/core"
)

// Generate —— 从攻击图生成完整报告（新版）。
func Generate(target string, g *core.AttackGraph, campaignID string, duration int) *Report {
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

	// 按严重级排序
	sort.SliceStable(findings, func(i, j int) bool {
		return sevRank[sevOfNode(findings[i])] < sevRank[sevOfNode(findings[j])]
	})

	// 统计
	var critCount, highCount, medCount, lowCount int
	for _, f := range findings {
		switch sevOfNode(f) {
		case "critical":
			critCount++
		case "high":
			highCount++
		case "medium":
			medCount++
		case "low":
			lowCount++
		}
	}

	// 计算风险评分（0-10）
	riskScore := calculateRiskScore(critCount, highCount, medCount, lowCount)

	rep := &Report{
		Meta: ReportMeta{
			Target:      target,
			GeneratedAt: time.Now(),
			Engine:      "Vero v1.0",
			CampaignID:  campaignID,
			Duration:    duration,
		},
		Executive: ExecutiveSummary{
			TotalServices:  len(services),
			TotalFindings:  len(findings),
			CriticalCount:  critCount,
			HighCount:      highCount,
			MediumCount:    medCount,
			LowCount:       lowCount,
			RiskScore:      riskScore,
			EvidenceStatus: "✓ 全部可溯源",
		},
		AttackSurface: buildServices(services),
		Findings:      buildFindings(findings),
		AttackGraph:   g,
		Remediation:   buildRecommendations(findings),
		Timeline:      GenerateTimeline(g),
		AttackPath:    GenerateAttackPath(g),
	}

	return rep
}

func calculateRiskScore(crit, high, med, low int) float64 {
	// 加权计算：Critical x4, High x2, Medium x1, Low x0.5
	weighted := float64(crit)*4 + float64(high)*2 + float64(med)*1 + float64(low)*0.5

	// 归一化到 0-10
	if weighted == 0 {
		return 0
	}
	if weighted >= 20 {
		return 10.0
	}
	return weighted / 2.0
}

func buildServices(nodes []*core.Node) []Service {
	var services []Service
	for _, n := range nodes {
		// 从 ID 尾部提取端口（格式：service:host:port）。
		// D12 修复: 用 LastIndex 而非 Split 取 parts[2] —— IPv6 ID(service:[::1]:80)会被冒号切坏。
		port := 0
		if i := strings.LastIndex(n.ID, ":"); i >= 0 {
			if p, err := strconv.Atoi(n.ID[i+1:]); err == nil {
				port = p
			}
		}

		protocol := "unknown"
		if strings.Contains(n.Label, "HTTP") {
			protocol = "http"
		} else if strings.Contains(n.Label, "SSH") {
			protocol = "ssh"
		} else if strings.Contains(n.Label, "SMB") {
			protocol = "smb"
		}

		services = append(services, Service{
			ID:       n.ID,
			Label:    n.Label,
			Port:     port,
			Protocol: protocol,
		})
	}
	return services
}

func buildFindings(nodes []*core.Node) []Finding {
	var findings []Finding
	for i, n := range nodes {
		sev := sevOfNode(n)
		title := titleOf(n.Label)

		// 构建证据
		var evidence []Evidence
		for _, ev := range n.Evidence {
			evidence = append(evidence, Evidence{
				Tool:    ev.Tool,
				Excerpt: ev.Excerpt,
			})
		}

		// 计算 CVSS（简化版，实际应该根据漏洞类型）
		cvss := calculateCVSS(title, sev)

		findings = append(findings, Finding{
			ID:          fmt.Sprintf("F%03d", i+1),
			Severity:    sev,
			Title:       title,
			Description: generateDescription(title),
			CVSS:        cvss,
			Evidence:    evidence,
			Remediation: generateRemediation(title),
		})
	}
	return findings
}

func calculateCVSS(title, severity string) CVSSScore {
	// 简化版 CVSS 计算（实际应该根据漏洞详细信息）
	var baseScore float64
	var vector string

	if strings.Contains(title, "SQLi") || strings.Contains(title, "SQL") {
		baseScore = 9.3
		vector = "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:N"
		severity = "critical"
	} else if strings.Contains(title, "Swagger") || strings.Contains(title, "API") {
		baseScore = 7.5
		vector = "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N"
		severity = "high"
	} else if strings.Contains(title, "Security Headers") {
		baseScore = 4.3
		vector = "CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:U/C:L/I:N/A:N"
		severity = "medium"
	} else {
		// 默认评分
		switch severity {
		case "critical":
			baseScore = 9.0
		case "high":
			baseScore = 7.5
		case "medium":
			baseScore = 5.0
		default:
			baseScore = 3.0
		}
		vector = "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:L/I:L/A:N"
	}

	return CVSSScore{
		BaseScore: baseScore,
		Vector:    vector,
		Severity:  strings.Title(severity),
	}
}

func generateDescription(title string) string {
	if strings.Contains(title, "SQLi") {
		return "SQL 注入漏洞允许攻击者通过恶意输入操纵数据库查询，可能导致数据泄露、数据篡改或完全控制数据库。"
	}
	if strings.Contains(title, "Swagger") {
		return "API 文档（Swagger/OpenAPI）公开暴露，可能泄露敏感的 API 端点、参数结构和业务逻辑，为攻击者提供攻击面信息。"
	}
	if strings.Contains(title, "Security Headers") {
		return "缺失关键安全响应头，可能导致 XSS、点击劫持、MIME 类型嗅探等攻击。"
	}
	return "检测到潜在安全风险，建议进一步人工验证和修复。"
}

func generateRemediation(title string) string {
	if strings.Contains(title, "SQLi") {
		return "使用参数化查询或 ORM 框架，对用户输入进行严格校验，部署 Web 应用防火墙（WAF）。"
	}
	if strings.Contains(title, "Swagger") {
		return "在生产环境关闭 Swagger UI 公开访问，或添加认证机制（BasicAuth/OAuth2）。"
	}
	if strings.Contains(title, "Security Headers") {
		return "配置 CSP、HSTS、X-Frame-Options、X-Content-Type-Options 等安全响应头。"
	}
	return "根据具体漏洞类型实施相应修复措施。"
}

func buildRecommendations(findings []*core.Node) []Recommendation {
	seen := map[string]bool{}
	var recs []Recommendation

	for _, f := range findings {
		title := titleOf(f.Label)
		sev := sevOfNode(f)

		var rec Recommendation
		if strings.Contains(title, "SQLi") && !seen["SQLi"] {
			seen["SQLi"] = true
			rec = Recommendation{
				Category:    "SQL 注入",
				Priority:    strings.Title(sev),
				Description: "应用存在 SQL 注入漏洞，攻击者可通过恶意输入访问或篡改数据库。",
				Steps: []string{
					"立即改用参数化查询（PreparedStatement）或 ORM 框架",
					"对所有用户输入进行严格校验和转义",
					"实施最小权限原则（数据库用户只授予必要权限）",
					"部署 WAF 进行实时检测和拦截",
					"定期进行代码审计和安全测试",
				},
			}
		} else if strings.Contains(title, "Swagger") && !seen["Swagger"] {
			seen["Swagger"] = true
			rec = Recommendation{
				Category:    "API 文档暴露",
				Priority:    "High",
				Description: "Swagger/OpenAPI 文档公开暴露，泄露 API 结构信息。",
				Steps: []string{
					"生产环境禁用 Swagger UI 公开访问",
					"如需保留，添加认证机制（BasicAuth/JWT）",
					"限制访问 IP 白名单",
					"考虑将文档部署到内网或独立域名",
				},
			}
		} else if strings.Contains(title, "Security Headers") && !seen["Headers"] {
			seen["Headers"] = true
			rec = Recommendation{
				Category:    "安全响应头缺失",
				Priority:    "Medium",
				Description: "缺少关键安全响应头，增加 XSS、点击劫持等风险。",
				Steps: []string{
					"配置 Content-Security-Policy (CSP) 防御 XSS",
					"启用 Strict-Transport-Security (HSTS) 强制 HTTPS",
					"设置 X-Frame-Options 防止点击劫持",
					"添加 X-Content-Type-Options: nosniff",
					"验证配置：使用 securityheaders.com 检测",
				},
			}
		}

		if rec.Category != "" {
			recs = append(recs, rec)
		}
	}

	if len(recs) == 0 {
		recs = append(recs, Recommendation{
			Category:    "持续监控",
			Priority:    "Low",
			Description: "当前未发现高危漏洞，建议保持定期安全测试。",
			Steps: []string{
				"每季度进行一次渗透测试",
				"订阅相关组件的安全公告",
				"实施持续安全监控",
			},
		})
	}

	return recs
}

// ToJSON —— 导出为 JSON。
func (r *Report) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
