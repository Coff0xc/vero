package scenarios

import (
	"strings"
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// TestCodeAuditPack —— 验证 CodeAuditPack 注册 + 工具规格 + Parser。
func TestCodeAuditPack(t *testing.T) {
	reg := tools.NewRegistry()
	m := NewManager()
	m.Register(reg, CodeAuditPack())

	// 1) 验证工具注册
	expectedTools := []string{"semgrep_scan", "bandit_scan", "eslint_security", "dependency_check"}
	for _, name := range expectedTools {
		if !reg.Has(name) {
			t.Errorf("工具 %s 未注册", name)
		}
	}

	// 2) 验证 Args 规格
	semgrepTool, _ := reg.Get("semgrep_scan")
	if len(semgrepTool.Args) < 1 || semgrepTool.Args[0].Name != "path" {
		t.Error("semgrep_scan 缺少 path 参数规格")
	}

	banditTool, _ := reg.Get("bandit_scan")
	if len(banditTool.Args) < 1 || banditTool.Args[0].Name != "path" {
		t.Error("bandit_scan 缺少 path 参数规格")
	}

	// 3) 验证 Parser (semgrep)
	mockSemgrepJSON := `{
  "results": [
    {
      "check_id": "python.lang.security.audit.dangerous-system-call",
      "path": "app.py",
      "start": {"line": 42, "col": 5},
      "end": {"line": 42},
      "extra": {
        "message": "Detected system() call with user input",
        "severity": "ERROR",
        "metadata": {
          "cwe": ["CWE-78"],
          "owasp": ["A03:2021"]
        },
        "lines": "os.system(user_input)"
      }
    }
  ]
}`
	obs := ParseSemgrep(mockSemgrepJSON, map[string]any{"path": "."})
	if len(obs) == 0 {
		t.Fatal("ParseSemgrep 应解析出至少 1 个观察")
	}
	if obs[0].Severity != "critical" {
		t.Errorf("ParseSemgrep 严重度映射错误: got %s, want critical", obs[0].Severity)
	}
	if !strings.Contains(obs[0].Label, "CWE-78") {
		t.Error("ParseSemgrep Label 未包含 CWE")
	}

	// 4) 验证 Parser (bandit)
	mockBanditJSON := `{
  "results": [
    {
      "test_id": "B105",
      "test_name": "hardcoded_password_string",
      "issue_text": "Possible hardcoded password: 'admin123'",
      "issue_severity": "HIGH",
      "issue_confidence": "MEDIUM",
      "filename": "config.py",
      "line_number": 15,
      "code": "PASSWORD = 'admin123'",
      "cwe": {"id": 259, "link": "https://cwe.mitre.org/data/definitions/259.html"}
    }
  ]
}`
	obsB := ParseBandit(mockBanditJSON, map[string]any{"path": "."})
	if len(obsB) == 0 {
		t.Fatal("ParseBandit 应解析出至少 1 个观察")
	}
	if obsB[0].Severity != "high" {
		t.Errorf("ParseBandit 严重度映射错误: got %s, want high", obsB[0].Severity)
	}
	if !strings.Contains(obsB[0].Label, "CWE-259") {
		t.Error("ParseBandit Label 未包含 CWE-259")
	}

	// 5) 验证指纹函数 (代码仓库特征)
	pack := CodeAuditPack()
	if !pack.Fingerprint(map[string]bool{"git-repo": true}) {
		t.Error("CodeAuditPack 应对 git-repo 服务指纹激活")
	}
	if !pack.Fingerprint(map[string]bool{"source-code": true}) {
		t.Error("CodeAuditPack 应对 source-code 服务指纹激活")
	}
	if pack.Fingerprint(map[string]bool{"http": true}) {
		t.Error("CodeAuditPack 不应对纯 http 激活")
	}
}

// TestCodeAuditParserEdgeCases —— 测试 Parser 边界情况。
func TestCodeAuditParserEdgeCases(t *testing.T) {
	// 1) 空 JSON
	obs := ParseSemgrep("{}", map[string]any{})
	if len(obs) != 0 {
		t.Error("空 JSON 应返回 0 个观察")
	}

	// 2) 非 JSON 输入
	obs = ParseSemgrep("Error: semgrep not found", map[string]any{})
	if obs != nil && len(obs) > 0 {
		t.Error("非 JSON 输入应返回 nil 或空切片")
	}

	// 3) 多个发现
	multiJSON := `{
  "results": [
    {
      "check_id": "rule1",
      "path": "a.py",
      "start": {"line": 1},
      "extra": {"message": "msg1", "severity": "WARNING", "lines": "code1", "metadata": {}}
    },
    {
      "check_id": "rule2",
      "path": "b.py",
      "start": {"line": 2},
      "extra": {"message": "msg2", "severity": "INFO", "lines": "code2", "metadata": {}}
    }
  ]
}`
	obs = ParseSemgrep(multiJSON, map[string]any{})
	if len(obs) != 2 {
		t.Errorf("应解析 2 个发现, got %d", len(obs))
	}

	// 4) 严重度映射
	testCases := []struct {
		input    string
		expected string
	}{
		{"CRITICAL", "critical"},
		{"ERROR", "critical"},
		{"HIGH", "high"},
		{"MEDIUM", "medium"},
		{"WARNING", "medium"},
		{"LOW", "low"},
		{"INFO", "low"},
		{"UNKNOWN", "medium"},
	}
	for _, tc := range testCases {
		if got := severityMap(tc.input); got != tc.expected {
			t.Errorf("severityMap(%s) = %s, want %s", tc.input, got, tc.expected)
		}
	}
}

// TestDependencyCheckParser —— 测试 OWASP Dependency-Check Parser。
func TestDependencyCheckParser(t *testing.T) {
	mockDepCheckJSON := `{
  "dependencies": [
    {
      "fileName": "jackson-databind-2.9.8.jar",
      "filePath": "/app/lib/jackson-databind-2.9.8.jar",
      "vulnerabilities": [
        {
          "name": "CVE-2019-12384",
          "severity": "CRITICAL",
          "cvssv3": {"baseScore": 9.8},
          "description": "Jackson Databind deserialization vulnerability"
        }
      ]
    },
    {
      "fileName": "log4j-core-2.14.0.jar",
      "filePath": "/app/lib/log4j-core-2.14.0.jar",
      "vulnerabilities": [
        {
          "name": "CVE-2021-44228",
          "severity": "CRITICAL",
          "cvssv3": {"baseScore": 10.0},
          "description": "Log4Shell JNDI injection"
        }
      ]
    }
  ]
}`

	obs := ParseDependencyCheck(mockDepCheckJSON, map[string]any{})
	if len(obs) != 2 {
		t.Fatalf("应解析 2 个 CVE, got %d", len(obs))
	}

	// 验证 CVE-2021-44228 (Log4Shell)
	var log4shellFound bool
	for _, o := range obs {
		if strings.Contains(o.Label, "CVE-2021-44228") {
			log4shellFound = true
			if o.Severity != "critical" {
				t.Errorf("Log4Shell 应为 critical, got %s", o.Severity)
			}
			if !strings.Contains(o.Excerpt, "10.0") {
				t.Error("Log4Shell Excerpt 应包含 CVSS 10.0")
			}
		}
	}
	if !log4shellFound {
		t.Error("未找到 CVE-2021-44228")
	}

	// 非 JSON 输入 (命令进度信息)
	obs = ParseDependencyCheck("Downloading NVD data feed...", map[string]any{})
	if len(obs) != 1 || obs[0].Kind != "info" {
		t.Error("非 JSON 输入应返回 info 提示")
	}
}
