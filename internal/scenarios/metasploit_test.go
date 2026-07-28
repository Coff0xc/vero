package scenarios

import (
	"strings"
	"testing"
)

// TestParseMSFSearch —— 验证 exploit 搜索结果解析。
func TestParseMSFSearch(t *testing.T) {
	output := `Found 3 exploits for 'CVE-2021-44228':
  exploit/linux/http/apache_log4j_rce
  exploit/windows/http/log4j_jndi_injection
  exploit/multi/http/log4shell_header_injection`

	obs := ParseMSFSearch(output, map[string]any{"query": "CVE-2021-44228"})

	// 应提取 3 个 exploit
	if len(obs) != 3 {
		t.Fatalf("应提取 3 个 exploit, 实际 %d", len(obs))
	}

	// 验证第一个 exploit
	if obs[0].Kind != "finding" {
		t.Errorf("应为 finding 类型, 实际 %s", obs[0].Kind)
	}
	if !strings.Contains(obs[0].Label, "apache_log4j_rce") {
		t.Errorf("Label 应含模块名, 实际 %s", obs[0].Label)
	}
	if !strings.Contains(obs[0].Excerpt, "exploit/linux/http/apache_log4j_rce") {
		t.Errorf("Excerpt 应含完整路径, 实际 %s", obs[0].Excerpt)
	}
}

// TestParseMSFExecute —— 验证 exploit 执行结果解析。
func TestParseMSFExecute(t *testing.T) {
	output := `Exploit exploit/linux/http/apache_log4j_rce launched (job_id: 1)
Target: 192.168.1.100
Waiting for session...`

	obs := ParseMSFExecute(output, map[string]any{"rhost": "192.168.1.100"})

	// 应提取 1 个 exploit 启动确认
	if len(obs) != 1 {
		t.Fatalf("应提取 1 个观测, 实际 %d", len(obs))
	}

	if obs[0].Kind != "finding" {
		t.Errorf("应为 finding 类型, 实际 %s", obs[0].Kind)
	}
	if !strings.Contains(obs[0].Label, "Exploit launched") {
		t.Errorf("Label 应含启动确认, 实际 %s", obs[0].Label)
	}
}

// TestParseMSFSessions —— 验证 session 列表解析。
func TestParseMSFSessions(t *testing.T) {
	output := `Active sessions (2):
  [1] shell @ 192.168.1.100:4444
  [2] meterpreter @ 192.168.1.101:5555`

	obs := ParseMSFSessions(output, map[string]any{})

	// 应提取 2 个 session
	if len(obs) != 2 {
		t.Fatalf("应提取 2 个 session, 实际 %d", len(obs))
	}

	// 验证第一个 session
	if obs[0].Kind != "shell" {
		t.Errorf("应为 shell 类型, 实际 %s", obs[0].Kind)
	}
	if !strings.Contains(obs[0].Label, "Active shell") {
		t.Errorf("Label 应含 shell 信息, 实际 %s", obs[0].Label)
	}
	if !strings.Contains(obs[0].Label, "192.168.1.100") {
		t.Errorf("Label 应含目标 IP, 实际 %s", obs[0].Label)
	}
}

// TestParseMSFSearchEmpty —— 空搜索结果。
func TestParseMSFSearchEmpty(t *testing.T) {
	output := `No exploits found for: unknown-cve`

	obs := ParseMSFSearch(output, map[string]any{"query": "unknown-cve"})

	if len(obs) != 0 {
		t.Errorf("无结果应返回空, 实际 %d", len(obs))
	}
}

// TestExploitPack —— 验证场景包注册。
func TestExploitPack(t *testing.T) {
	pack := ExploitPack()

	if pack.Name != "exploit" {
		t.Errorf("包名应为 exploit, 实际 %s", pack.Name)
	}

	// 应有 3 个工具
	if len(pack.Tools) != 3 {
		t.Fatalf("应有 3 个工具, 实际 %d", len(pack.Tools))
	}

	// 验证工具存在
	toolNames := make(map[string]bool)
	for _, tool := range pack.Tools {
		toolNames[tool.Name] = true
	}

	required := []string{"msf_search", "msf_execute", "msf_get_sessions"}
	for _, name := range required {
		if !toolNames[name] {
			t.Errorf("缺失工具: %s", name)
		}
	}

	// 验证指纹函数
	services := map[string]bool{"finding": true}
	if !pack.Fingerprint(services) {
		t.Error("finding 应激活漏洞利用场景包")
	}
}
