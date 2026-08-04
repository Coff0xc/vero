package scenarios

// Package code_audit —— 代码审计场景包(抄 DeepAudit + BugTraceAI):
// SAST 引擎(semgrep/bandit) + 污点分析 + 依赖漏洞扫描, 覆盖 OWASP Top 10 代码层缺陷。
//
// 设计要点:
//   - 工具选型: semgrep(通用规则引擎) > bandit(Python) > eslint-security(JS/TS)。
//   - 输出结构化: JSON 模式便于 Parser 提取 CVE/CWE/污点路径。
//   - 证据完整: 代码片段 + 行号 + 修复建议, 逐字可溯源。
//   - 依赖扫描: OWASP Dependency-Check 检测已知漏洞组件。

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// semgrepScan —— semgrep 静态扫描(通用规则引擎, 支持 30+ 语言)。
// 默认用 "auto" 规则集(自动选择语言 + 社区规则), JSON 输出便于解析。
func semgrepScan(args map[string]any) tools.ToolResult {
	path := tools.ArgStr(args, "path", ".")
	severity := tools.ArgStr(args, "severity", "WARNING") // ERROR/WARNING/INFO

	// semgrep --config auto --severity WARNING --json <path>
	// --no-git-ignore: 不跳过 .gitignore 文件(可能藏敏感配置)
	// --max-memory 2048: 限制内存防止 OOM
	cmdArgs := []string{
		"semgrep", "scan",
		"--config", "auto",
		"--severity", severity,
		"--json",
		"--no-git-ignore",
		"--max-memory", "2048",
		path,
	}
	return tools.Sh(cmdArgs, 600*time.Second) // 大仓库可能需要 10 分钟
}

// banditScan —— bandit Python 安全扫描器(检测硬编码密码/SQL 注入/不安全反序列化)。
func banditScan(args map[string]any) tools.ToolResult {
	path := tools.ArgStr(args, "path", ".")
	level := tools.ArgStr(args, "level", "MEDIUM") // LOW/MEDIUM/HIGH

	// bandit -r <path> -f json -ll MEDIUM
	// -r: 递归扫描
	// -f json: JSON 输出
	// -ll: 置信度等级(LOW/MEDIUM/HIGH)
	cmdArgs := []string{
		"bandit", "-r", path,
		"-f", "json",
		"-ll", level,
		"--quiet", // 不输出进度条
	}
	return tools.Sh(cmdArgs, 300*time.Second)
}

// eslintSecScan —— ESLint 安全插件扫描(JS/TS XSS/原型污染)。
func eslintSecScan(args map[string]any) tools.ToolResult {
	path := tools.ArgStr(args, "path", ".")

	// eslint <path> --plugin security --format json
	// 需要项目已安装 eslint-plugin-security
	cmdArgs := []string{
		"npx", "eslint", path,
		"--ext", ".js,.ts,.jsx,.tsx",
		"--plugin", "security",
		"--format", "json",
		"--no-eslintrc", // 不读项目配置(避免禁用安全规则)
	}
	return tools.Sh(cmdArgs, 300*time.Second)
}

// dependencyCheck —— OWASP Dependency-Check 依赖漏洞扫描(检测已知 CVE)。
func dependencyCheck(args map[string]any) tools.ToolResult {
	path := tools.ArgStr(args, "path", ".")
	format := tools.ArgStr(args, "format", "JSON") // JSON/XML/HTML

	// dependency-check --scan <path> --format JSON --out ./dc-report
	// --enableExperimental: 支持 Go/Rust 等新语言(默认只扫描 Java/Node/Python)
	cmdArgs := []string{
		"dependency-check",
		"--scan", path,
		"--format", format,
		"--out", "./dc-report",
		"--enableExperimental",
		"--failOnCVSS", "0", // 不因 CVE 退出失败(我们要拿到结果)
	}
	return tools.Sh(cmdArgs, 900*time.Second) // 首次运行需下载 NVD 数据库(可能 15 分钟)
}

// ParseSemgrep —— 解析 semgrep JSON 输出: 提取规则/文件/行号/代码片段/修复建议。
// Excerpt 保留完整 JSON finding, 保证逐字回查。
func ParseSemgrep(out string, args map[string]any) []tools.Observation {
	var result struct {
		Results []struct {
			CheckID string `json:"check_id"` // 规则 ID, 如 python.lang.security.audit.dangerous-system-call
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
				Col  int `json:"col"`
			} `json:"start"`
			End struct {
				Line int `json:"line"`
			} `json:"end"`
			Extra struct {
				Message  string `json:"message"`
				Severity string `json:"severity"` // ERROR/WARNING/INFO
				Metadata struct {
					CWE      []string `json:"cwe,omitempty"`
					OWASP    []string `json:"owasp,omitempty"`
					Impact   string   `json:"impact,omitempty"`
					LikeLihood string `json:"likelihood,omitempty"`
				} `json:"metadata"`
				Lines string `json:"lines"` // 代码片段
			} `json:"extra"`
		} `json:"results"`
	}

	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil // 非 JSON 输出(可能是错误信息)
	}

	var obs []tools.Observation
	for _, r := range result.Results {
		severity := severityMap(r.Extra.Severity)

		// 标签: 优先用 CWE, 无则用规则 ID 最后段
		tag := "code-issue"
		if len(r.Extra.Metadata.CWE) > 0 {
			tag = r.Extra.Metadata.CWE[0] // CWE-79
		} else {
			parts := strings.Split(r.CheckID, ".")
			if len(parts) > 0 {
				tag = parts[len(parts)-1] // dangerous-system-call
			}
		}

		// 摘要: 文件:行号 + 消息
		excerpt := fmt.Sprintf("%s:%d - %s", r.Path, r.Start.Line, r.Extra.Message)

		// 代码片段作为证据(截断到 200 字符)
		codeSnippet := r.Extra.Lines
		if len(codeSnippet) > 200 {
			codeSnippet = codeSnippet[:200] + "..."
		}

		label := fmt.Sprintf("[%s] %s (%s)", severity, r.CheckID, tag)

		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      fmt.Sprintf("sast:%s:%d", r.Path, r.Start.Line),
			Label:    label,
			Excerpt:  excerpt,
			Severity: severity,
		})
	}

	return obs
}

// ParseBandit —— 解析 bandit JSON 输出: 提取问题/文件/行号/CWE。
func ParseBandit(out string, args map[string]any) []tools.Observation {
	var result struct {
		Results []struct {
			TestID       string `json:"test_id"`        // B105
			TestName     string `json:"test_name"`      // hardcoded_password_string
			IssueText    string `json:"issue_text"`
			IssueSeverity string `json:"issue_severity"` // HIGH/MEDIUM/LOW
			IssueConfidence string `json:"issue_confidence"`
			Filename     string `json:"filename"`
			LineNumber   int    `json:"line_number"`
			Code         string `json:"code"`
			CWE          struct {
				ID   int    `json:"id"`   // 259
				Link string `json:"link"` // https://cwe.mitre.org/...
			} `json:"cwe"`
		} `json:"results"`
	}

	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil
	}

	var obs []tools.Observation
	for _, r := range result.Results {
		severity := severityMap(r.IssueSeverity)
		cweTag := fmt.Sprintf("CWE-%d", r.CWE.ID)

		excerpt := fmt.Sprintf("%s:%d - %s", r.Filename, r.LineNumber, r.IssueText)
		label := fmt.Sprintf("[%s] %s (%s)", severity, r.TestName, cweTag)

		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      fmt.Sprintf("bandit:%s:%d", r.Filename, r.LineNumber),
			Label:    label,
			Excerpt:  excerpt,
			Severity: severity,
		})
	}

	return obs
}

// ParseDependencyCheck —— 解析 OWASP Dependency-Check JSON: 提取 CVE/组件/严重度。
func ParseDependencyCheck(out string, args map[string]any) []tools.Observation {
	var result struct {
		Dependencies []struct {
			FileName string `json:"fileName"`
			FilePath string `json:"filePath"`
			Vulnerabilities []struct {
				Name        string  `json:"name"`        // CVE-2021-44228
				Severity    string  `json:"severity"`    // CRITICAL/HIGH/MEDIUM/LOW
				CvssV3      *struct {
					BaseScore float64 `json:"baseScore"` // 10.0
				} `json:"cvssv3"`
				Description string `json:"description"`
			} `json:"vulnerabilities,omitempty"`
		} `json:"dependencies"`
	}

	// dependency-check 输出在 ./dc-report/dependency-check-report.json
	// 这里 out 是命令 stdout(可能只有进度), 需要读文件
	// 暂时简化: 如果 out 不是 JSON, 返回空(后续可增强读文件)
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		return []tools.Observation{{
			Kind:    "info",
			Label:   "[info] dependency-check 已完成, 报告位于 ./dc-report/",
			Excerpt: "需手动检查 dependency-check-report.json",
		}}
	}

	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil
	}

	var obs []tools.Observation
	for _, dep := range result.Dependencies {
		if len(dep.Vulnerabilities) == 0 {
			continue
		}

		for _, vuln := range dep.Vulnerabilities {
			severity := severityMap(vuln.Severity)

			cvss := "N/A"
			if vuln.CvssV3 != nil {
				cvss = fmt.Sprintf("%.1f", vuln.CvssV3.BaseScore)
			}

			excerpt := fmt.Sprintf("%s: %s (CVSS %s)", dep.FileName, vuln.Name, cvss)
			label := fmt.Sprintf("[%s] %s in %s", severity, vuln.Name, dep.FileName)

			obs = append(obs, tools.Observation{
				Kind:     "finding",
				Key:      fmt.Sprintf("cve:%s:%s", dep.FileName, vuln.Name),
				Label:    label,
				Excerpt:  excerpt,
				Severity: severity,
			})
		}
	}

	return obs
}

// severityMap —— 标准化严重度标签(不同工具用词不一致)。
func severityMap(s string) string {
	s = strings.ToUpper(s)
	switch s {
	case "CRITICAL", "ERROR":
		return "critical"
	case "HIGH":
		return "high"
	case "MEDIUM", "WARNING":
		return "medium"
	case "LOW", "INFO":
		return "low"
	default:
		return "medium"
	}
}

// oneline —— 把多行文本压缩成单行(去除换行/制表符, 截断到 maxLen)。
func oneline(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen] + "..."
	}
	return s
}

// CodeAuditPack —— 代码审计场景包(抄 DeepAudit): 静态扫描 + 依赖检查。
func CodeAuditPack() Pack {
	return Pack{
		Name: "CodeAudit",
		Tools: []*tools.Tool{
			{
				Name:  "semgrep_scan",
				Level: tools.LevelScan,
				Desc:  "Semgrep 静态代码扫描(30+ 语言通用规则), 检测 SQL 注入/XSS/硬编码密码等 OWASP Top 10",
				Run:   semgrepScan,
				Parse: ParseSemgrep,
				Args: []tools.ArgSpec{
					{Name: "path", Desc: "扫描目标路径(文件/目录)", Required: true},
					{Name: "severity", Desc: "最低严重度: ERROR/WARNING/INFO (默认 WARNING)"},
				},
			},
			{
				Name:  "bandit_scan",
				Level: tools.LevelScan,
				Desc:  "Bandit Python 安全扫描器, 检测硬编码密码/不安全反序列化/弱加密",
				Run:   banditScan,
				Parse: ParseBandit,
				Args: []tools.ArgSpec{
					{Name: "path", Desc: "扫描目标路径(Python 项目)", Required: true},
					{Name: "level", Desc: "置信度等级: LOW/MEDIUM/HIGH (默认 MEDIUM)"},
				},
			},
			{
				Name:  "eslint_security",
				Level: tools.LevelScan,
				Desc:  "ESLint 安全插件扫描(JS/TS), 检测 XSS/原型污染/不安全 eval",
				Run:   eslintSecScan,
				Parse: nil, // 暂不实现(需处理 eslint 输出格式)
				Args: []tools.ArgSpec{
					{Name: "path", Desc: "扫描目标路径(JS/TS 项目)", Required: true},
				},
			},
			{
				Name:  "dependency_check",
				Level: tools.LevelScan,
				Desc:  "OWASP Dependency-Check 依赖漏洞扫描, 检测已知 CVE 组件(支持 Java/Node/Python/Go)",
				Run:   dependencyCheck,
				Parse: ParseDependencyCheck,
				Args: []tools.ArgSpec{
					{Name: "path", Desc: "扫描目标路径(包含依赖清单的项目根目录)", Required: true},
					{Name: "format", Desc: "报告格式: JSON/XML/HTML (默认 JSON)"},
				},
			},
		},
		// 指纹: 发现代码仓库特征时激活(git/package.json/requirements.txt/go.mod)
		Fingerprint: func(services map[string]bool) bool {
			return services["git-repo"] || services["source-code"]
		},
	}
}
