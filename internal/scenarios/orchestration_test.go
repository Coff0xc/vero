package scenarios

import (
	"strings"
	"testing"
)

func TestOrchestrationPack(t *testing.T) {
	pack := OrchestrationPack()
	if len(pack) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(pack))
	}

	// 验证工具名称
	names := []string{"parallel_scan", "chain_exploit", "aggregate_findings"}
	for i, tool := range pack {
		if tool.Name != names[i] {
			t.Errorf("tool %d: expected %s, got %s", i, names[i], tool.Name)
		}
	}
}

func TestParallelScan(t *testing.T) {
	// 测试单个IP
	args := map[string]any{
		"targets": "192.168.1.1",
		"workers": "2",
	}
	res := parallelScan(args)
	if !res.Success {
		t.Fatalf("parallel_scan failed: %s", res.Stderr)
	}
	if !strings.Contains(res.Stdout, "并行扫描完成") {
		t.Errorf("missing summary in output")
	}
}

func TestParallelScanCIDR(t *testing.T) {
	// 测试CIDR扩展
	args := map[string]any{
		"targets": "192.168.1.0/30", // 应扩展为 4 个 IP
		"workers": "2",
	}
	res := parallelScan(args)
	if !res.Success {
		t.Fatalf("parallel_scan CIDR failed: %s", res.Stderr)
	}
	// 验证扩展了多个目标
	if strings.Count(res.Stdout, "[Worker") < 2 {
		t.Errorf("expected multiple worker outputs, got: %s", res.Stdout)
	}
}

func TestChainExploit(t *testing.T) {
	args := map[string]any{
		"target": "192.168.1.100",
		"depth":  "3",
	}
	res := chainExploit(args)
	if !res.Success {
		t.Fatalf("chain_exploit failed: %s", res.Stderr)
	}
	// 验证阶段输出
	if !strings.Contains(res.Stdout, "阶段 1") {
		t.Errorf("missing stage 1 in output")
	}
	if !strings.Contains(res.Stdout, "阶段 2") {
		t.Errorf("missing stage 2 in output")
	}
	if !strings.Contains(res.Stdout, "阶段 3") {
		t.Errorf("missing stage 3 in output")
	}
}

func TestAggregateFindings(t *testing.T) {
	args := map[string]any{
		"campaign_ids":  "c1,c2,c3",
		"dedup_window": "600",
	}
	res := aggregateFindings(args)
	if !res.Success {
		t.Fatalf("aggregate_findings failed: %s", res.Stderr)
	}
	if !strings.Contains(res.Stdout, "唯一发现") {
		t.Errorf("missing unique findings count")
	}
}

func TestParseParallelScan(t *testing.T) {
	output := `并行扫描完成: 2 个目标, 2 个工作者
[Worker 0] 192.168.1.1: Scanning ports 80,443
  Open ports: 22 (ssh), 80 (http), 443 (https)
---
[Worker 1] 192.168.1.2: Scanning ports 80,443
  Open ports: 80 (http)
`
	obs := ParseParallelScan(output, nil)
	if len(obs) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}
	if obs[0].Kind != "host" {
		t.Errorf("expected kind=host, got %s", obs[0].Kind)
	}
}

func TestParseChainExploit(t *testing.T) {
	output := `链式利用目标: test.com (深度 3)

=== 阶段 1: 侦察 ===
目标: test.com
端口扫描完成（模拟）

=== 阶段 2: 漏洞扫描 ===
Nuclei 扫描完成（模拟）

=== 阶段 3: 自动利用 ===
检测到 SQL 注入，尝试利用（模拟）
`
	obs := ParseChainExploit(output, nil)
	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}
	if obs[2].Kind != "exploit" {
		t.Errorf("expected kind=exploit, got %s", obs[2].Kind)
	}
}

func TestExpandTargets(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"192.168.1.1", 1},
		{"192.168.1.1,192.168.1.2", 2},
		{"192.168.1.0/30", 4},
		{"192.168.1.1-3", 3},
	}

	for _, tt := range tests {
		result := expandTargets(tt.input)
		if len(result) != tt.expected {
			t.Errorf("expandTargets(%q): expected %d, got %d (%v)",
				tt.input, tt.expected, len(result), result)
		}
	}
}
