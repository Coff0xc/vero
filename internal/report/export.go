package report

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// ToMarkdown —— 导出为专业 Markdown 报告。
func (r *Report) ToMarkdown() string {
	var b strings.Builder

	// 标题和元数据
	fmt.Fprintf(&b, "# 渗透测试报告 — %s\n\n", r.Meta.Target)
	fmt.Fprintf(&b, "> **生成时间**: %s  \n", r.Meta.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "> **引擎**: %s  \n", r.Meta.Engine)
	fmt.Fprintf(&b, "> **战役 ID**: `%s`  \n", r.Meta.CampaignID)
	fmt.Fprintf(&b, "> **测试时长**: %d 秒\n\n", r.Meta.Duration)

	b.WriteString("---\n\n")

	// 执行摘要
	b.WriteString("## 📊 执行摘要\n\n")
	fmt.Fprintf(&b, "| 指标 | 数值 |\n")
	fmt.Fprintf(&b, "|------|------|\n")
	fmt.Fprintf(&b, "| 目标 | `%s` |\n", r.Meta.Target)
	fmt.Fprintf(&b, "| 发现服务 | **%d** |\n", r.Executive.TotalServices)
	fmt.Fprintf(&b, "| 总发现数 | **%d** |\n", r.Executive.TotalFindings)
	fmt.Fprintf(&b, "| 🔴 Critical | **%d** |\n", r.Executive.CriticalCount)
	fmt.Fprintf(&b, "| 🟠 High | **%d** |\n", r.Executive.HighCount)
	fmt.Fprintf(&b, "| 🟡 Medium | **%d** |\n", r.Executive.MediumCount)
	fmt.Fprintf(&b, "| 🔵 Low | **%d** |\n", r.Executive.LowCount)
	fmt.Fprintf(&b, "| 风险评分 | **%.1f/10** |\n", r.Executive.RiskScore)
	fmt.Fprintf(&b, "| 证据完整性 | %s |\n\n", r.Executive.EvidenceStatus)

	// 风险等级指示器
	riskLevel := "低风险"
	if r.Executive.RiskScore >= 8.0 {
		riskLevel = "🔴 **高风险** - 需要立即修复"
	} else if r.Executive.RiskScore >= 5.0 {
		riskLevel = "🟠 **中风险** - 建议尽快修复"
	} else if r.Executive.RiskScore >= 2.0 {
		riskLevel = "🟡 **低风险** - 建议关注"
	} else {
		riskLevel = "✅ **安全** - 保持监控"
	}
	fmt.Fprintf(&b, "### 综合风险等级\n\n%s\n\n", riskLevel)

	b.WriteString("---\n\n")

	// 攻击面
	if len(r.AttackSurface) > 0 {
		b.WriteString("## 🌐 攻击面分析\n\n")
		b.WriteString("| 端口 | 协议 | 服务详情 |\n")
		b.WriteString("|------|------|----------|\n")
		for _, svc := range r.AttackSurface {
			fmt.Fprintf(&b, "| %d | `%s` | %s |\n", svc.Port, svc.Protocol, svc.Label)
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")

	// 漏洞详情
	b.WriteString("## 🔍 漏洞详情（按严重级排序）\n\n")
	if len(r.Findings) == 0 {
		b.WriteString("✅ **未发现确认漏洞**\n\n")
	} else {
		for i, finding := range r.Findings {
			severityEmoji := map[string]string{
				"critical": "🔴",
				"high":     "🟠",
				"medium":   "🟡",
				"low":      "🔵",
				"info":     "ℹ️",
			}
			emoji := severityEmoji[finding.Severity]

			fmt.Fprintf(&b, "### %s %d. [%s] %s\n\n", emoji, i+1, strings.ToUpper(finding.Severity), finding.Title)

			// CVSS 评分
			fmt.Fprintf(&b, "**CVSS v3.1 评分**: %.1f (%s)  \n", finding.CVSS.BaseScore, finding.CVSS.Severity)
			fmt.Fprintf(&b, "**CVSS 向量**: `%s`\n\n", finding.CVSS.Vector)

			// 描述
			fmt.Fprintf(&b, "**描述**:  \n%s\n\n", finding.Description)

			// 证据
			if len(finding.Evidence) > 0 {
				b.WriteString("**证据**（逐字来自工具输出）:\n\n")
				for _, ev := range finding.Evidence {
					fmt.Fprintf(&b, "```text\n[工具: %s]\n%s\n```\n\n", ev.Tool, ev.Excerpt)
				}
			}

			// 修复建议
			fmt.Fprintf(&b, "**修复建议**:  \n%s\n\n", finding.Remediation)
			b.WriteString("---\n\n")
		}
	}

	// 修复优先级
	b.WriteString("## 🛠️ 修复建议优先级\n\n")
	for i, rec := range r.Remediation {
		priorityEmoji := map[string]string{
			"Critical": "🔴",
			"High":     "🟠",
			"Medium":   "🟡",
			"Low":      "🔵",
		}
		emoji := priorityEmoji[rec.Priority]

		fmt.Fprintf(&b, "### %s %d. %s [优先级: %s]\n\n", emoji, i+1, rec.Category, rec.Priority)
		fmt.Fprintf(&b, "%s\n\n", rec.Description)

		b.WriteString("**修复步骤**:\n\n")
		for j, step := range rec.Steps {
			fmt.Fprintf(&b, "%d. %s\n", j+1, step)
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")

	// 附录
	b.WriteString("## 📎 附录\n\n")
	b.WriteString("### 关于 REDCELL\n\n")
	b.WriteString("REDCELL 是一个 AI 驱动的自主红队渗透测试平台，采用证据驱动机制确保所有发现可溯源、可验证。\n\n")
	b.WriteString("**核心特性**:\n")
	b.WriteString("- ✅ 证据逐字溯源（反幻觉机制）\n")
	b.WriteString("- ✅ 智能安全门控（L0-L4 分级）\n")
	b.WriteString("- ✅ 动态攻击图规划\n")
	b.WriteString("- ✅ 持续安全验证\n\n")

	b.WriteString("### 免责声明\n\n")
	b.WriteString("本报告由自动化工具生成，仅供授权渗透测试使用。建议结合人工复核确认所有发现。使用本工具进行未授权测试属于违法行为。\n\n")

	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "*报告生成时间: %s*\n", r.Meta.GeneratedAt.Format("2006-01-02 15:04:05"))

	return b.String()
}

// HTML 模板
const htmlTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>渗透测试报告 - {{.Meta.Target}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
            line-height: 1.6;
            color: #333;
            background: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        h1 { color: #1a1a1a; margin-bottom: 10px; font-size: 32px; }
        h2 { color: #2c3e50; margin-top: 30px; margin-bottom: 15px; padding-bottom: 10px; border-bottom: 2px solid #3498db; }
        h3 { color: #34495e; margin-top: 20px; margin-bottom: 10px; }
        .meta {
            background: #ecf0f1;
            padding: 15px;
            border-radius: 5px;
            margin-bottom: 30px;
            font-size: 14px;
        }
        .meta strong { color: #2c3e50; }
        table {
            width: 100%;
            border-collapse: collapse;
            margin: 20px 0;
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        th {
            background: #3498db;
            color: white;
            font-weight: 600;
        }
        .badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 600;
            text-transform: uppercase;
        }
        .critical { background: #e74c3c; color: white; }
        .high { background: #e67e22; color: white; }
        .medium { background: #f39c12; color: white; }
        .low { background: #3498db; color: white; }
        .info { background: #95a5a6; color: white; }
        .risk-score {
            text-align: center;
            padding: 30px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border-radius: 8px;
            margin: 20px 0;
        }
        .risk-score h3 { color: white; border: none; }
        .risk-score .score { font-size: 48px; font-weight: bold; margin: 10px 0; }
        .evidence {
            background: #f8f9fa;
            padding: 15px;
            border-left: 4px solid #3498db;
            margin: 10px 0;
            font-family: "Courier New", monospace;
            font-size: 13px;
            overflow-x: auto;
        }
        .remediation {
            background: #e8f5e9;
            padding: 15px;
            border-left: 4px solid #4caf50;
            margin: 10px 0;
        }
        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
            text-align: center;
            color: #7f8c8d;
            font-size: 14px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔒 渗透测试报告</h1>

        <div class="meta">
            <strong>目标:</strong> {{.Meta.Target}} <br>
            <strong>生成时间:</strong> {{.Meta.GeneratedAt.Format "2006-01-02 15:04:05"}} <br>
            <strong>引擎:</strong> {{.Meta.Engine}} <br>
            <strong>战役 ID:</strong> {{.Meta.CampaignID}} <br>
            <strong>测试时长:</strong> {{.Meta.Duration}} 秒
        </div>

        <h2>📊 执行摘要</h2>
        <table>
            <tr><th>指标</th><th>数值</th></tr>
            <tr><td>发现服务</td><td><strong>{{.Executive.TotalServices}}</strong></td></tr>
            <tr><td>总发现数</td><td><strong>{{.Executive.TotalFindings}}</strong></td></tr>
            <tr><td>🔴 Critical</td><td><span class="badge critical">{{.Executive.CriticalCount}}</span></td></tr>
            <tr><td>🟠 High</td><td><span class="badge high">{{.Executive.HighCount}}</span></td></tr>
            <tr><td>🟡 Medium</td><td><span class="badge medium">{{.Executive.MediumCount}}</span></td></tr>
            <tr><td>🔵 Low</td><td><span class="badge low">{{.Executive.LowCount}}</span></td></tr>
            <tr><td>证据完整性</td><td>{{.Executive.EvidenceStatus}}</td></tr>
        </table>

        <div class="risk-score">
            <h3>综合风险评分</h3>
            <div class="score">{{printf "%.1f" .Executive.RiskScore}}/10</div>
            <p>{{if ge .Executive.RiskScore 8.0}}🔴 高风险 - 需要立即修复{{else if ge .Executive.RiskScore 5.0}}🟠 中风险 - 建议尽快修复{{else if ge .Executive.RiskScore 2.0}}🟡 低风险 - 建议关注{{else}}✅ 安全 - 保持监控{{end}}</p>
        </div>

        {{if .AttackSurface}}
        <h2>🌐 攻击面分析</h2>
        <table>
            <tr><th>端口</th><th>协议</th><th>服务详情</th></tr>
            {{range .AttackSurface}}
            <tr>
                <td>{{.Port}}</td>
                <td><code>{{.Protocol}}</code></td>
                <td>{{.Label}}</td>
            </tr>
            {{end}}
        </table>
        {{end}}

        <h2>🔍 漏洞详情</h2>
        {{if .Findings}}
            {{range $i, $f := .Findings}}
            <h3>{{add $i 1}}. <span class="badge {{$f.Severity}}">{{upper $f.Severity}}</span> {{$f.Title}}</h3>
            <p><strong>CVSS v3.1:</strong> {{printf "%.1f" $f.CVSS.BaseScore}} ({{$f.CVSS.Severity}})</p>
            <p><strong>描述:</strong> {{$f.Description}}</p>

            {{if $f.Evidence}}
            <p><strong>证据:</strong></p>
            {{range $f.Evidence}}
            <div class="evidence">
                <strong>[工具: {{.Tool}}]</strong><br>
                {{.Excerpt}}
            </div>
            {{end}}
            {{end}}

            <div class="remediation">
                <strong>🛠️ 修复建议:</strong><br>
                {{$f.Remediation}}
            </div>
            {{end}}
        {{else}}
            <p>✅ <strong>未发现确认漏洞</strong></p>
        {{end}}

        <h2>🛠️ 修复优先级</h2>
        {{range $i, $r := .Remediation}}
        <h3>{{add $i 1}}. {{$r.Category}} [优先级: {{$r.Priority}}]</h3>
        <p>{{$r.Description}}</p>
        <p><strong>修复步骤:</strong></p>
        <ol>
            {{range $r.Steps}}
            <li>{{.}}</li>
            {{end}}
        </ol>
        {{end}}

        <div class="footer">
            <p><strong>关于 REDCELL</strong></p>
            <p>AI 驱动的自主红队渗透测试平台 | 证据驱动 · 安全可控 · 生产就绪</p>
            <p style="margin-top: 10px; font-size: 12px;">
                ⚠️ 免责声明：本报告仅供授权渗透测试使用。建议结合人工复核。
            </p>
        </div>
    </div>
</body>
</html>`

// ToHTML —— 导出为专业 HTML 报告。
func (r *Report) ToHTML() (string, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"upper": strings.ToUpper,
	}).Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, r); err != nil {
		return "", fmt.Errorf("渲染模板失败: %w", err)
	}

	return buf.String(), nil
}
